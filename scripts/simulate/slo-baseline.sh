#!/usr/bin/env bash
# EXP-33 / ADR-003.6 — SLO calibration: steady load, then snapshot the
# baseline envelope from Prometheus and append it to SLO-BASELINE.md.
#
# Usage:  bash scripts/simulate/slo-baseline.sh
# Env:    SLO_VUS=10 SLO_DURATION=10m PROM_URL=http://localhost:9090
#         TARGET=compose|cluster   (picks the sim-load flavor; default compose)
set -euo pipefail

SLO_VUS="${SLO_VUS:-10}"
SLO_DURATION="${SLO_DURATION:-10m}"
PROM_URL="${PROM_URL:-http://localhost:9090}"
TARGET="${TARGET:-compose}"
DOC="documentation/experiments/SLO-BASELINE.md"

cd "$(dirname "$0")/../.."

echo "==> checking Prometheus at ${PROM_URL}"
curl -sf "${PROM_URL}/-/ready" >/dev/null || {
  echo "Prometheus not reachable at ${PROM_URL}."
  echo "compose: make up   ·   kind: kubectl -n lab-obs port-forward svc/<prometheus> 9090:9090"
  exit 1
}

echo "==> steady load: ${SLO_VUS} VUs for ${SLO_DURATION} (${TARGET})"
if [ "$TARGET" = "cluster" ]; then
  make cluster-sim-load SIM_VUS="${SLO_VUS}" SIM_DURATION="${SLO_DURATION}"
else
  make sim-load SIM_VUS="${SLO_VUS}" SIM_DURATION="${SLO_DURATION}"
fi

echo "==> waiting 60s for queues to drain before the snapshot"
sleep 60

# Instant-query helper: prints "metric-label value" lines for a PromQL expr.
q() {
  local expr="$1"
  curl -sf --get "${PROM_URL}/api/v1/query" --data-urlencode "query=${expr}" \
    | python3 -c '
import json,sys
d=json.load(sys.stdin)
for r in d.get("data",{}).get("result",[]):
    m=r.get("metric",{})
    label=m.get("queue") or m.get("pod") or m.get("__name__") or ""
    v=r.get("value",[None,"nan"])[1]
    try: v=f"{float(v):.4f}"
    except ValueError: pass
    print(f"{label or \"value\"}: {v}")
'
}

WIN="${SLO_DURATION}"
{
  echo ""
  echo "## Calibration run — $(date -u '+%Y-%m-%d %H:%MZ')"
  echo ""
  echo "Conditions: target=${TARGET}, VUs=${SLO_VUS}, duration=${SLO_DURATION}, host=$(uname -n)"
  echo ""
  echo '```'
  echo "-- API latency quantiles (s, over the run window) --"
  for Q in 0.50 0.95 0.99; do
    printf "p%s " "${Q#0.}"
    q "histogram_quantile(${Q}, sum by (le) (rate(api_http_request_duration_seconds_bucket[${WIN}])))"
  done
  echo "-- API request rate (req/s) --"
  q "sum(rate(api_http_requests_total[${WIN}]))"
  echo "-- API 5xx ratio --"
  q "sum(rate(api_http_requests_total{status=~\"5..\"}[${WIN}])) / sum(rate(api_http_requests_total[${WIN}]))"
  echo "-- Worker drain rates (msg/s) --"
  q "sum by (__name__) (rate({__name__=~\".+_processing_success_total\"}[${WIN}]))"
  echo "-- Peak work-queue depth --"
  q "max_over_time(rabbitmq_queue_messages{queue=~\".*-processing\"}[${WIN}])"
  echo "-- Pod memory working set (bytes, cluster only; empty on compose) --"
  q "max by (pod) (container_memory_working_set_bytes{namespace=\"lab-core\", container!=\"\"})" || true
  echo '```'
  echo ""
  echo "_Next: transfer these into the worksheet above, add margins, update the PrometheusRule thresholds._"
} >> "${DOC}"

echo "==> appended results block to ${DOC}"
