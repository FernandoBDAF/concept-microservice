#!/usr/bin/env bash
# bootstrap.sh — smoke test proving a fresh copy of this template builds, boots,
# and exercises the auth pattern end to end against a real Postgres:
#
#   register -> login -> validate -> refresh (rotate) -> reuse rejected -> logout
#
# It also verifies the JWKS document is served. Deliberately breakable (EXP-81):
# if session rotation, reuse detection, or validation regresses, an assertion
# below fails and the script exits non-zero.
#
# Requires: docker (with compose v2) + curl. openssl is optional — present, the
# run exercises RS256/JWKS; absent, it falls back to HS256. Nothing to run first;
# the script owns its throwaway stack and tears it down on exit.
#
# Usage:  test/bootstrap.sh            (from the template root or anywhere)
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$HERE/compose.bootstrap.yml"
PROJECT="auth-bootstrap-$$"
BASE_URL="http://localhost:3000"
EMAIL="smoke+$$@example.com"
PASSWORD="smoke-password-123"

DC=(docker compose -p "$PROJECT" -f "$COMPOSE_FILE")

log()  { printf '\033[36m[bootstrap]\033[0m %s\n' "$*"; }
fail() { printf '\033[31m[bootstrap] FAIL:\033[0m %s\n' "$*" >&2; exit 1; }
pass() { printf '\033[32m[bootstrap] ok:\033[0m %s\n' "$*"; }

cleanup() {
  log "tearing down"
  "${DC[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

command -v docker >/dev/null 2>&1 || fail "docker is required"
command -v curl   >/dev/null 2>&1 || fail "curl is required"

# --- Optional RS256/JWKS: mint a keypair so the smoke covers the JWKS path -----
if command -v openssl >/dev/null 2>&1; then
  log "generating RS256 keypair (JWKS path)"
  eval "$("$HERE/../scripts/gen-keys.sh")"   # exports JWT_PRIVATE_KEY / JWT_PUBLIC_KEY
  export JWT_PRIVATE_KEY JWT_PUBLIC_KEY
  RS256=1
else
  log "openssl not found — using HS256 fallback"
  RS256=0
fi

# --- Bring up the stack --------------------------------------------------------
log "building + starting stack (this can take a minute on first run)"
"${DC[@]}" up -d --build

# --- Wait for readiness (defensive: retries with backoff) ----------------------
log "waiting for $BASE_URL/ready"
ready=0
for i in $(seq 1 60); do
  if curl -fsS "$BASE_URL/ready" >/dev/null 2>&1; then ready=1; break; fi
  sleep 2
done
[ "$ready" = 1 ] || { "${DC[@]}" logs auth-service | tail -50; fail "service did not become ready"; }
pass "service ready"

# --- Tiny JSON field extractor (no jq dependency) ------------------------------
# Pulls the first string value for a given key from a flat-ish JSON blob.
json_field() { sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" <<<"$2" | head -1; }

req() { # req METHOD PATH [DATA] [AUTH_BEARER] -> sets HTTP + BODY
  local method="$1" path="$2" data="${3:-}" bearer="${4:-}"
  local args=(-sS -o /tmp/bootstrap.body -w '%{http_code}' -X "$method" "$BASE_URL$path"
              -H 'Content-Type: application/json')
  [ -n "$data" ]   && args+=(-d "$data")
  [ -n "$bearer" ] && args+=(-H "Authorization: Bearer $bearer")
  HTTP="$(curl "${args[@]}")"
  BODY="$(cat /tmp/bootstrap.body)"
}

# --- 0. JWKS document ----------------------------------------------------------
req GET /.well-known/jwks.json
[ "$HTTP" = 200 ] || fail "jwks: expected 200, got $HTTP"
if [ "$RS256" = 1 ]; then
  grep -q '"kty":"RSA"' <<<"$BODY" || grep -q '"kty": "RSA"' <<<"$BODY" \
    || fail "jwks: expected an RSA key in RS256 mode, got: $BODY"
  pass "JWKS serves an RSA signing key"
else
  pass "JWKS reachable (HS256 mode: keys array may be empty)"
fi

# --- 1. Register ---------------------------------------------------------------
req POST /v1/users "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
[ "$HTTP" = 201 ] || fail "register: expected 201, got $HTTP ($BODY)"
pass "registered $EMAIL"

# --- 2. Login ------------------------------------------------------------------
req POST /v1/auth/login "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
[ "$HTTP" = 200 ] || fail "login: expected 200, got $HTTP ($BODY)"
ACCESS1="$(json_field access_token "$BODY")"
REFRESH1="$(json_field refresh_token "$BODY")"
[ -n "$ACCESS1" ] && [ -n "$REFRESH1" ] || fail "login: missing tokens ($BODY)"
pass "logged in (got access + refresh tokens)"

# --- 3. Validate ---------------------------------------------------------------
req POST /v1/auth/token/validate "{\"token\":\"$ACCESS1\"}"
[ "$HTTP" = 200 ] || fail "validate: expected 200, got $HTTP ($BODY)"
grep -q '"valid":true' <<<"$BODY" || grep -q '"valid": true' <<<"$BODY" \
  || fail "validate: expected valid:true ($BODY)"
pass "access token validates"

# --- 4. Refresh (rotation) -----------------------------------------------------
req POST /v1/auth/token/refresh "{\"refresh_token\":\"$REFRESH1\"}"
[ "$HTTP" = 200 ] || fail "refresh: expected 200, got $HTTP ($BODY)"
ACCESS2="$(json_field access_token "$BODY")"
REFRESH2="$(json_field refresh_token "$BODY")"
[ -n "$ACCESS2" ] && [ -n "$REFRESH2" ] || fail "refresh: missing rotated tokens ($BODY)"
[ "$REFRESH2" != "$REFRESH1" ] || fail "refresh: token did not rotate"
pass "refresh rotated the session (new token pair)"

# --- 5. Reuse of the OLD refresh token must be rejected (theft signal) ---------
req POST /v1/auth/token/refresh "{\"refresh_token\":\"$REFRESH1\"}"
[ "$HTTP" = 401 ] || fail "reuse: expected 401 for rotated-out token, got $HTTP ($BODY)"
pass "reuse of rotated-out refresh token rejected (401)"

# --- 6. Logout -----------------------------------------------------------------
req POST /v1/auth/logout "" "$ACCESS2"
[ "$HTTP" = 200 ] || fail "logout: expected 200, got $HTTP ($BODY)"
pass "logout succeeded"

printf '\033[32m[bootstrap] ALL CHECKS PASSED\033[0m\n'
