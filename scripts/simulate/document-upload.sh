#!/usr/bin/env bash
# EXPERIMENTS.md EXP-11: document pipeline end-to-end.
# register → login → create profile → upload file → poll status → download URL
# Requires the compose stack up (make up). Host tools: curl, python3.
set -euo pipefail

AUTH="${AUTH_URL:-http://localhost:3000}"
API="${API_URL:-http://localhost:8080}"
# Cluster mode (EXP-20): pass TLS/resolve flags, e.g.
#   CURL_OPTS="-k --resolve api.lab.local:443:127.0.0.1 --resolve auth.lab.local:443:127.0.0.1"
CURL="curl -sf ${CURL_OPTS:-}"
EMAIL="doc-demo-$(date +%s)@lab.dev"
PASSWORD="Doc-demo-pass-123!"

json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(d$1)"; }

echo "== 1/6 register ${EMAIL}"
$CURL -X POST "$AUTH/v1/users" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" >/dev/null

echo "== 2/6 login"
TOKEN=$($CURL -X POST "$AUTH/v1/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | json "['data']['access_token']")

echo "== 3/6 create profile"
PROFILE_ID=$($CURL -X POST "$API/api/v1/profiles" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"first_name\":\"Doc\",\"last_name\":\"Demo\",\"email\":\"$EMAIL\",\"bio\":\"document E2E\"}" \
  | json "['id']")
echo "   profile: $PROFILE_ID"

echo "== 4/6 upload document"
# BSD mktemp can't do suffixes; the .txt extension matters to the API's
# file-type allow-list, so create a temp dir and name the file inside it.
TMPDIR_DOC=$(mktemp -d)
TMPFILE="$TMPDIR_DOC/lab-document.txt"
trap 'rm -rf "$TMPDIR_DOC"' EXIT
printf 'Lab document pipeline E2E test\ncreated: %s\nprofile: %s\n' \
  "$(date)" "$PROFILE_ID" > "$TMPFILE"
UPLOAD=$($CURL -X POST "$API/api/v1/documents/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -F "profile_id=$PROFILE_ID" -F "file=@$TMPFILE")
echo "   $UPLOAD"
DOC_ID=$(echo "$UPLOAD" | json "['document_id']")
echo "   document: $DOC_ID"

echo "== 5/6 status (poll x5 — graphrag consumes document.process)"
for _ in 1 2 3 4 5; do
  sleep 2
  STATUS=$($CURL "$API/api/v1/documents/$DOC_ID/status" -H "Authorization: Bearer $TOKEN" || true)
  echo "   $STATUS"
done

echo "== 6/6 download endpoint"
$CURL "$API/api/v1/documents/$DOC_ID/download" -H "Authorization: Bearer $TOKEN" | head -c 200
echo
echo "E2E OK — check MinIO console (localhost:9001) bucket documents-raw, and: make logs S=graphrag-service"
