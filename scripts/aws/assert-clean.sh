#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# assert-clean.sh (HANDOFF §8, ADR-006.4) — teardown assertion for EXP-50.
#
# Queries the resource-groups tagging API for everything tagged
# `project=coppice-lab` in ONE region, subtracts the persistent base-stack
# allowlist, and fails if anything remains. Run after `make aws-down`: a clean
# exit proves no session resources leaked; a non-zero exit lists the stragglers
# (the reaper is the backstop for those, but aws-down should already be clean).
#
# The allowlist is authoritative on the `stack=base` tag — every base-stack
# resource (ECR, zone, budget, reaper, ntfy-notifier, OIDC) inherits it via the
# provider default_tags. Untagged persistent bootstrap resources (tfstate
# bucket, lock table) don't carry project=coppice-lab at all, so they never
# appear here; they're listed in the name allowlist as belt-and-suspenders.
#
# Usage:
#   scripts/aws/assert-clean.sh [--region R] [--profile P]
# Region/profile resolve from flags, then AWS_REGION/AWS_PROFILE, then the
# terraform.tfvars conventions (us-east-1 / lab).
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

REGION="${AWS_REGION:-us-east-1}"
PROFILE="${AWS_PROFILE:-lab}"

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
    echo "assert-clean: unknown argument '$1' (see --help)" >&2
    exit 2
    ;;
  esac
done

# ── Runtime requirements ─────────────────────────────────────────────────────
if ! command -v aws >/dev/null 2>&1; then
  echo "assert-clean: the AWS CLI is required but not found on PATH." >&2
  echo "  Install it (https://docs.aws.amazon.com/cli/) and configure the" >&2
  echo "  '$PROFILE' profile, then re-run." >&2
  exit 3
fi
if ! command -v python3 >/dev/null 2>&1; then
  echo "assert-clean: python3 is required to parse the tagging API output." >&2
  exit 3
fi

echo "assert-clean: querying project=coppice-lab in ${REGION} (profile ${PROFILE})…" >&2

RAW="$(aws resourcegroupstaggingapi get-resources \
  --tag-filters Key=project,Values=coppice-lab \
  --region "$REGION" \
  --profile "$PROFILE" \
  --output json)"

# ── Filter: drop base-stack + explicitly-allowlisted persistent ARNs ─────────
# python3 does the parsing (jq not assumed present). Anything left is a leak.
LEAKS="$(printf '%s' "$RAW" | python3 -c '
import json, re, sys

data = json.load(sys.stdin)

# Persistent base-stack ARN patterns (belt-and-suspenders behind stack=base).
ALLOW = [
    r":s3:::coppice-lab-tfstate-",                       # tfstate bucket
    r":dynamodb:[^:]*:[^:]*:table/coppice-lab-tf-lock$", # lock table
    r":ecr:[^:]*:[^:]*:repository/coppice-lab/",         # ECR repos
    r":route53:::hostedzone/",                           # lab zone
    r":budgets::[^:]*:budget/",                          # monthly budget
    r":sns:[^:]*:[^:]*:coppice-lab-budget$",             # budget SNS topic
    r":lambda:[^:]*:[^:]*:function:coppice-lab-reaper$",
    r":lambda:[^:]*:[^:]*:function:coppice-lab-ntfy-notifier$",
    r":events:[^:]*:[^:]*:rule/coppice-lab-reaper-hourly$",
    r":iam::[^:]*:role/coppice-lab-reaper$",
    r":iam::[^:]*:role/coppice-lab-ntfy-notifier$",
    r":iam::[^:]*:oidc-provider/",                       # GitHub OIDC provider
    r":iam::[^:]*:role/coppice-lab-deploy",              # OIDC deploy role (base/oidc.tf)
]
allow_re = [re.compile(p) for p in ALLOW]

leaks = []
for res in data.get("ResourceTagMappingList", []):
    arn = res.get("ResourceARN", "")
    tags = {t.get("Key"): t.get("Value") for t in res.get("Tags", [])}
    if tags.get("stack") == "base":
        continue  # authoritative: base stack is always persistent
    if any(r.search(arn) for r in allow_re):
        continue
    leaks.append(arn)

for arn in sorted(leaks):
    print(arn)
')"

if [ -n "$LEAKS" ]; then
  echo "assert-clean: FAIL — session resources still tagged project=coppice-lab:" >&2
  printf '  %s\n' $LEAKS >&2
  echo "assert-clean: $(printf '%s\n' $LEAKS | wc -l | tr -d ' ') leaked resource(s). Re-run aws-down or wait for the reaper." >&2
  exit 1
fi

echo "clean: no session resources remain (base-stack allowlist only)."
exit 0
