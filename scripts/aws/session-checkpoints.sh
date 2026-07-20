#!/usr/bin/env bash
# make aws-deploy tail: walk the AWS_SESSION.md session-start checkpoints and
# print live status for each. Non-fatal — a red checkpoint means "look here",
# the session goes on (EXP-50 asserts the hard pass/fail).
set -uo pipefail
cd "$(dirname "$0")/../.."

step() { printf '\n\033[1;34m== %s\033[0m\n' "$*"; }

step "checkpoint 1/4 — nodes Ready"
kubectl get nodes -o wide

step "checkpoint 2/4 — migrations + rds-bootstrap Complete"
kubectl -n lab-infra get jobs
kubectl -n lab-infra wait --for=condition=complete job --all --timeout=300s || true

step "checkpoint 3/4 — lab-core rollouts"
for d in $(kubectl -n lab-core get deploy -o name); do
  kubectl -n lab-core rollout status "$d" --timeout=180s || true
done
kubectl -n lab-core get pods

step "checkpoint 4/4 — ALB ingress + DNS"
kubectl -n lab-core get ingress -o wide
cat <<'EOF'

Next (manual, ~2-5 min for ALB provisioning + external-dns records):
  1. Wait for the ingress ADDRESS above (the shared coppice-lab ALB).
  2. external-dns publishes api./auth. records into the lab zone; verify:
       dig +short api.<lab_domain>
  3. Smoke: curl -s https://api.<lab_domain>/health   → 200
  4. Then run the drill of the day (EXP-50..55) — record actual $ cost
     in the write-up (AWS_SESSION.md cost check).
EOF
