#!/usr/bin/env bash
# `make up` runs this first: seed .env with an RSA-2048 keypair so the compose
# auth-service signs RS256 (ADR-009.1). Keys are written base64(PEM) single-line
# — the exact shape the k8s Secret uses (scripts/cluster/init-secrets.sh) and
# the auth-service consumes (JWT_PRIVATE_KEY / JWT_PUBLIC_KEY). Absent keys →
# auth falls back to HS256 with JWT_SECRET (designed). Idempotent: leaves an
# existing keypair untouched (compose reads .env, which is gitignored).
set -euo pipefail
cd "$(dirname "$0")/../.."

ENVFILE=".env"
touch "$ENVFILE"

if grep -q '^JWT_PRIVATE_KEY=' "$ENVFILE"; then
  echo "gen-jwt-keys: JWT_PRIVATE_KEY already present in $ENVFILE — leaving as is"
  exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$tmp/private.pem" 2>/dev/null
openssl pkey -in "$tmp/private.pem" -pubout -out "$tmp/public.pem" 2>/dev/null

# base64 single-line: GNU base64 wraps at 76 cols, so strip newlines (BSD
# base64 emits one line already — tr is a harmless no-op there).
b64() { base64 < "$1" | tr -d '\n'; }

{
  echo "JWT_PRIVATE_KEY=$(b64 "$tmp/private.pem")"
  echo "JWT_PUBLIC_KEY=$(b64 "$tmp/public.pem")"
} >> "$ENVFILE"
echo "gen-jwt-keys: wrote JWT_PRIVATE_KEY / JWT_PUBLIC_KEY (base64 PEM) to $ENVFILE"
