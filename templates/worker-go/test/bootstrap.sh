#!/usr/bin/env bash
# bootstrap.sh — the worker-go template's smoke test (v8-HANDOFF §5, EXP-81).
#
# Proves a fresh copy of the template builds and behaves: compose up
# (rabbitmq + redis + worker), publish 10 valid messages (all consumed), then
# 1 poison message (missing required payload field → unretryable → DLQ),
# asserting the counts through the RabbitMQ management API, then compose down.
#
# It is DELIBERATELY BREAKABLE (EXP-81): break the processor, the routing keys,
# or the generated topology and the assertions below must fail. Defensive
# waits/retries make a *green* run trustworthy, not lenient.
#
# Requires: docker (compose v2), python3, curl. No Go toolchain needed at run
# time (the worker is built in-image).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COMPOSE="docker compose -f compose.snippet.yml"
MGMT="http://guest:guest@localhost:15672/api"
EXCHANGE="example-tasks"
WORK_QUEUE="example-processing"
DLQ="example-processing.dlq"
RK="example.task"
N_VALID=10

log()  { printf '\033[1;34m[bootstrap]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[bootstrap] FAIL:\033[0m %s\n' "$*" >&2; exit 1; }

cleanup() {
  log "tearing down"
  $COMPOSE down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

# retry CMD... up to N times, SLEEP secs apart, until it exits 0.
retry() {
  local n="$1" sleep_s="$2"; shift 2
  local i
  for ((i = 1; i <= n; i++)); do
    if "$@"; then return 0; fi
    sleep "$sleep_s"
  done
  return 1
}

# queue_field QUEUE FIELD -> prints the numeric field (e.g. messages,
# messages_ready) from the management API, or 0 if the queue is absent.
queue_field() {
  local q="$1" field="$2"
  curl -fsS "$MGMT/queues/%2F/$q" 2>/dev/null \
    | python3 -c "import json,sys;print(json.load(sys.stdin).get(sys.argv[1],0))" "$field" 2>/dev/null \
    || echo 0
}

publish() {
  # publish PAYLOAD_JSON -> POSTs to the exchange; asserts the broker routed it.
  local payload="$1"
  local body
  body=$(python3 -c '
import json,sys
env = sys.argv[1]
print(json.dumps({"properties": {"delivery_mode": 2}, "routing_key": sys.argv[2],
                  "payload": env, "payload_encoding": "string"}))
' "$payload" "$RK")
  local routed
  routed=$(curl -fsS -u guest:guest -H "content-type: application/json" \
    -d "$body" "$MGMT/exchanges/%2F/$EXCHANGE/publish" \
    | python3 -c "import json,sys;print(json.load(sys.stdin).get('routed'))")
  [[ "$routed" == "True" ]] || fail "message not routed by $EXCHANGE (rk=$RK) — topology mismatch?"
}

# ── 0. preconditions ────────────────────────────────────────────────────────
command -v docker  >/dev/null || fail "docker not found"
command -v python3 >/dev/null || fail "python3 not found"
command -v curl    >/dev/null || fail "curl not found"

# ── 1. generate the broker topology the compose stack loads at boot ──────────
log "generating deploy/rabbitmq/definitions.json (--full)"
python3 scripts/generate-definitions.py --pipeline example:example.task \
  --full -o deploy/rabbitmq/definitions.json

# ── 2. bring the stack up ────────────────────────────────────────────────────
log "compose up (rabbitmq + redis + worker)"
$COMPOSE up -d --build

log "waiting for the management API"
retry 30 2 curl -fsS "$MGMT/overview" >/dev/null || fail "management API never came up"

log "waiting for the work queue to be declared from definitions.json"
retry 30 2 bash -c "curl -fsS '$MGMT/queues/%2F/$WORK_QUEUE' >/dev/null" \
  || fail "$WORK_QUEUE not present — definitions.json not loaded?"

log "waiting for the worker to report ready"
retry 30 2 bash -c "curl -fsS http://localhost:8080/ready >/dev/null" \
  || fail "worker /ready never succeeded"

# ── 3. publish 10 valid messages ────────────────────────────────────────────
log "publishing $N_VALID valid messages"
for ((i = 1; i <= N_VALID; i++)); do
  publish "{\"id\":\"msg-$i\",\"type\":\"example.task\",\"payload\":{\"target\":\"widget-$i\"}}"
done

log "waiting for the work queue to drain (all consumed)"
drained() { [[ "$(queue_field "$WORK_QUEUE" messages)" == "0" ]]; }
retry 30 2 drained || fail "work queue did not drain — expected all $N_VALID consumed"

# ── 4. publish 1 poison message → must land in the DLQ ───────────────────────
log "publishing 1 poison message (missing required payload.target)"
publish "{\"id\":\"poison-1\",\"type\":\"example.task\",\"payload\":{\"note\":\"no target\"}}"

log "waiting for exactly 1 message in the DLQ"
dlq_is_one() { [[ "$(queue_field "$DLQ" messages)" == "1" ]]; }
retry 30 2 dlq_is_one || fail "expected 1 message in $DLQ, got $(queue_field "$DLQ" messages)"

# ── 5. final assertions ──────────────────────────────────────────────────────
work_left=$(queue_field "$WORK_QUEUE" messages)
dlq_count=$(queue_field "$DLQ" messages)
[[ "$work_left" == "0" ]] || fail "work queue should be empty, has $work_left"
[[ "$dlq_count" == "1" ]] || fail "DLQ should hold exactly 1 poison message, has $dlq_count"

log "OK — $N_VALID consumed, work queue empty, 1 poison in DLQ"
