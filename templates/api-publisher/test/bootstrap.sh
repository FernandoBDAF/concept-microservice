#!/usr/bin/env bash
# bootstrap.sh — proves a fresh copy of the api-publisher builds and honors its
# core promise: it publishes work RELIABLY, closing the crash window between
# "committed to the DB" and "published to the broker".
#
# Two smokes:
#   A. Happy path        — POST a task → outbox row → relay publishes →
#                          RabbitMQ management API shows the message.
#   B. Crash window (the point) — a task is committed to the outbox while the
#                          relay is DEAD (process crashed after commit, before
#                          publish). On restart the relay MUST publish it. This
#                          is the EXP-42 invariant (authored; live run pending).
#
# Deliberately breakable (EXP-81): break the relay, the routing map, or the
# outbox transaction and one of the assertions below fails loudly.
#
# Requires: docker compose, curl, jq. Not run in CI's unit-test job — this is
# the compose-based smoke (lab CI runs it behind a `templates/` path filter).
set -euo pipefail

cd "$(dirname "$0")/.."

COMPOSE=(docker compose -f compose.snippet.yml)
API=http://localhost:8080
MGMT=http://guest:guest@localhost:15672/api
QUEUE=example-processing
VHOST=%2F   # url-encoded "/"

log()  { printf '\n\033[1;34m== %s\033[0m\n' "$*"; }
fail() { printf '\033[1;31mFAIL: %s\033[0m\n' "$*" >&2; dump; exit 1; }
dump() { "${COMPOSE[@]}" ps || true; "${COMPOSE[@]}" logs --tail=40 api || true; }

cleanup() { log "tearing down"; "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT

# --- helpers ---------------------------------------------------------------

# psql_scalar <sql>: run SQL, echo the scalar result trimmed.
psql_scalar() {
  "${COMPOSE[@]}" exec -T postgres psql -U postgres -d api_db -tAc "$1" | tr -d '[:space:]'
}

queue_depth() { curl -fsS "$MGMT/queues/$VHOST/$QUEUE" | jq -r '.messages // 0'; }

# wait_for <desc> <timeout_s> <predicate...> : poll the predicate (a function
# or command, run IN THIS SHELL) until it succeeds or the timeout elapses.
wait_for() {
  local desc="$1" timeout="$2"; shift 2
  local deadline=$(( SECONDS + timeout ))
  until "$@" >/dev/null 2>&1; do
    (( SECONDS < deadline )) || fail "timed out waiting for: $desc"
    sleep 1
  done
}

# --- predicates (exit 0 = ready) -------------------------------------------
pg_ready()     { "${COMPOSE[@]}" exec -T postgres pg_isready -U postgres -d api_db; }
rmq_up()       { curl -fsS "$MGMT/overview" >/dev/null; }
api_healthy()  { curl -fsS "$API/healthz" | jq -e '.status=="ok"' >/dev/null; }
api_down()     { ! curl -fsS "$API/healthz" >/dev/null 2>&1; }
some_row_sent()      { [ "$(psql_scalar "SELECT count(*) FROM outbox WHERE sent_at IS NOT NULL;")" -ge 1 ]; }
marker_sent()        { [ "$(psql_scalar "SELECT count(*) FROM outbox WHERE envelope->>'id'='$1' AND sent_at IS NOT NULL;")" -eq 1 ]; }
queue_gt()           { [ "$(queue_depth)" -gt "$1" ]; }

post_task() {
  curl -fsS -o /dev/null -w '%{http_code}' \
    -X POST "$API/tasks" -H 'Content-Type: application/json' \
    -d '{"routing_key":"example.task","type":"example.task","payload":{"resource_id":"r-'"$1"'"}}'
}

# --- bring the stack up -----------------------------------------------------

log "building + starting stack"
"${COMPOSE[@]}" up -d --build

wait_for "postgres ready"       90 pg_ready
wait_for "rabbitmq management"  90 rmq_up
wait_for "api healthy"          90 api_healthy

# ===========================================================================
# A. Happy path
# ===========================================================================
log "A. happy path — POST a task, expect it published"

BEFORE=$(queue_depth)
code=$(post_task a) || fail "POST /tasks failed"
[ "$code" = "202" ] || fail "expected 202 from POST /tasks, got $code"

total=$(psql_scalar "SELECT count(*) FROM outbox WHERE routing_key='example.task';")
[ "${total:-0}" -ge 1 ] || fail "expected an outbox row, found $total"

wait_for "relay marks a row sent"       20 some_row_sent
wait_for "message visible in RabbitMQ"  20 queue_gt "$BEFORE"
log "A. PASS — task published and visible in the broker"

# ===========================================================================
# B. Crash window — committed-but-unpublished survives and publishes later
# ===========================================================================
log "B. crash window — kill the relay between commit and publish"

# Stop the API (and with it the in-process relay). This is the crash: the
# process is gone. We then create the state a crash-after-commit leaves behind:
# an outbox row committed to the DB but never published.
"${COMPOSE[@]}" stop api >/dev/null
wait_for "api stopped" 30 api_down

MARKER="crash-window-$$"
psql_scalar "INSERT INTO outbox (routing_key, envelope) VALUES ('example.task', '{\"id\":\"$MARKER\",\"type\":\"example.task\",\"payload\":{}}');" >/dev/null
pending=$(psql_scalar "SELECT count(*) FROM outbox WHERE envelope->>'id'='$MARKER' AND sent_at IS NULL;")
[ "${pending:-0}" -eq 1 ] || fail "expected exactly one pending (unpublished) row, found $pending"
log "committed one row while the relay was dead; it is unpublished"

BEFORE=$(queue_depth)
log "restart the relay — it MUST publish the orphaned row"
"${COMPOSE[@]}" start api >/dev/null
wait_for "api healthy again"            60 api_healthy
wait_for "orphaned row published"       30 marker_sent "$MARKER"
wait_for "broker received recovered msg" 30 queue_gt "$BEFORE"

log "B. PASS — the crash window is closed: commit survived and published on recovery"
log "ALL SMOKES PASSED"
