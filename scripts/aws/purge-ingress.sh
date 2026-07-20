#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# purge-ingress.sh (HANDOFF §8, ADR-006.6) — ALB teardown BEFORE terraform.
#
# The aws-load-balancer-controller provisions the shared ALB + its security
# groups OUTSIDE terraform state, in response to the Ingress objects. If
# `terraform destroy` runs first it deletes the controller (and the VPC) while
# the ALB/SGs still dangle: VPC deletion then fails with DependencyViolation and
# the ~$16/mo ALB leaks. So we delete the Ingresses first and WAIT for the
# controller to reap the ALB, THEN let terraform destroy proceed.
#
# Wired into `make aws-down` ahead of `terraform destroy`. Best-effort by design:
# if the session cluster is unreachable (the common case — no session deployed)
# it warns loudly and exits 0 so the destroy can still run.
#
# Usage:
#   scripts/aws/purge-ingress.sh [--region R] [--profile P]
# Region/profile resolve from flags, then AWS_REGION/AWS_PROFILE, then the
# terraform.tfvars conventions (us-east-1 / lab).
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

REGION="${AWS_REGION:-us-east-1}"
PROFILE="${AWS_PROFILE:-lab}"

# Session-stack project tag (deploy/aws/session/main.tf provider default_tags);
# the ALB controller stamps it onto the ALB it creates (controller defaultTags).
PROJECT_TAG_KEY="project"
PROJECT_TAG_VALUE="coppice-lab"

CLUSTER_INFO_TIMEOUT=15 # s — quick reachability probe
INGRESS_DELETE_TIMEOUT=180 # s — kubectl --wait on ingress deletion
ALB_REAP_TIMEOUT=300 # s — poll budget for the controller to delete the ALB
ALB_REAP_INTERVAL=10 # s — poll cadence

while [ $# -gt 0 ]; do
  case "$1" in
  --region)
    REGION="$2"
    shift 2
    ;;
  --profile)
    PROFILE="$2"
    shift 2
    ;;
  -h | --help)
    grep '^#' "$0" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
  *)
    echo "purge-ingress: unknown argument '$1' (see --help)" >&2
    exit 2
    ;;
  esac
done

# ── Runtime requirements ─────────────────────────────────────────────────────
if ! command -v kubectl >/dev/null 2>&1; then
  echo "purge-ingress: kubectl not found on PATH — cannot delete Ingresses." >&2
  echo "  WARNING: skipping ALB purge; terraform destroy may fail with" >&2
  echo "  DependencyViolation if a session ALB is still up. Continuing." >&2
  exit 0
fi
if ! command -v aws >/dev/null 2>&1; then
  echo "purge-ingress: the AWS CLI is required but not found on PATH." >&2
  exit 3
fi

# ── Context guard ────────────────────────────────────────────────────────────
# `kubectl delete ingress --all -A` acts on the CURRENT context — which on a dev
# machine may be the kind cluster (aws-down does not run aws-kubeconfig first).
# Only purge when the current context is the EKS session cluster; the
# update-kubeconfig context name is the cluster ARN (…:cluster/coppice-lab).
current_ctx="$(kubectl config current-context 2>/dev/null || true)"
case "$current_ctx" in
*:cluster/coppice-lab)
  : # EKS session cluster — safe to purge
  ;;
*)
  echo "purge-ingress: WARNING — current kubectl context is '${current_ctx:-<none>}'," >&2
  echo "  not the EKS session cluster (…:cluster/coppice-lab). Refusing to delete" >&2
  echo "  Ingresses on it. Run 'make aws-kubeconfig' first if a session is up;" >&2
  echo "  otherwise terraform destroy may hit DependencyViolation. Continuing." >&2
  exit 0
  ;;
esac

# ── Reachability guard ───────────────────────────────────────────────────────
# If the current kubeconfig context can't reach a cluster, there is nothing to
# purge (or no session was deployed). Warn loudly and let destroy proceed.
if ! kubectl cluster-info --request-timeout="${CLUSTER_INFO_TIMEOUT}s" >/dev/null 2>&1; then
  echo "purge-ingress: WARNING — session cluster is unreachable (no kubeconfig" >&2
  echo "  context, or the cluster is already gone). Skipping Ingress deletion." >&2
  echo "  If a session ALB is still up, terraform destroy may hit" >&2
  echo "  DependencyViolation — check the AWS console / reaper. Continuing." >&2
  exit 0
fi

echo "purge-ingress: deleting all Ingress objects (waiting up to ${INGRESS_DELETE_TIMEOUT}s)…" >&2
kubectl delete ingress --all -A --wait "--timeout=${INGRESS_DELETE_TIMEOUT}s" || {
  echo "purge-ingress: WARNING — ingress deletion did not complete cleanly." >&2
  echo "  Continuing to the ALB reap poll anyway." >&2
}

# ── Wait for the controller to reap the ALB ──────────────────────────────────
# Poll ELBv2 for any load balancer still tagged ${PROJECT_TAG_KEY}=${PROJECT_TAG_VALUE}.
# The controller deletes the ALB it created once the Ingresses are gone.
echo "purge-ingress: waiting for the ALB controller to reap the ${PROJECT_TAG_VALUE} ALB (up to ${ALB_REAP_TIMEOUT}s)…" >&2

reaped=0
deadline=$(( $(date +%s) + ALB_REAP_TIMEOUT ))
while [ "$(date +%s)" -lt "$deadline" ]; do
  lb_arns="$(aws elbv2 describe-load-balancers \
    --region "$REGION" --profile "$PROFILE" \
    --query 'LoadBalancers[].LoadBalancerArn' --output text 2>/dev/null || true)"

  if [ -z "$lb_arns" ]; then
    reaped=1
    break
  fi

  # describe-tags is limited to 20 ARNs/call; the lab only runs one shared ALB,
  # but batch defensively so a stray count never truncates the check.
  tagged=""
  # shellcheck disable=SC2086 # word-splitting the tab/space-separated ARN list is intended
  set -- $lb_arns
  while [ $# -gt 0 ]; do
    batch=""
    n=0
    while [ $# -gt 0 ] && [ $n -lt 20 ]; do
      batch="$batch $1"
      shift
      n=$((n + 1))
    done
    # shellcheck disable=SC2086
    match="$(aws elbv2 describe-tags \
      --resource-arns $batch \
      --region "$REGION" --profile "$PROFILE" \
      --query "TagDescriptions[?Tags[?Key=='${PROJECT_TAG_KEY}' && Value=='${PROJECT_TAG_VALUE}']].ResourceArn" \
      --output text 2>/dev/null || true)"
    if [ -n "$match" ]; then
      tagged="$tagged $match"
    fi
  done

  if [ -z "${tagged// /}" ]; then
    reaped=1
    break
  fi

  echo "purge-ingress: ALB still present, waiting ${ALB_REAP_INTERVAL}s…" >&2
  sleep "$ALB_REAP_INTERVAL"
done

if [ "$reaped" -eq 1 ]; then
  echo "purge-ingress: ALB reaped — safe to terraform destroy." >&2
else
  echo "purge-ingress: WARNING — an ALB tagged ${PROJECT_TAG_KEY}=${PROJECT_TAG_VALUE} is" >&2
  echo "  STILL present after ${ALB_REAP_TIMEOUT}s. terraform destroy may fail with" >&2
  echo "  DependencyViolation on the VPC, and the ALB (~\$16/mo) may leak. Check the" >&2
  echo "  EC2 → Load Balancers console; the reaper is the backstop. Continuing." >&2
fi

exit 0
