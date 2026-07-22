#!/usr/bin/env bash
# gen-keys.sh — generate an RS256 signing keypair for the auth service.
#
# JWKS/RS256 (ADR-009.1): the service signs access tokens with a private key and
# publishes the public half at /.well-known/jwks.json so consumers verify
# locally. This script mints a fresh 2048-bit RSA keypair and emits the two env
# vars the config loader understands (JWT_PRIVATE_KEY / JWT_PUBLIC_KEY), each as
# single-line base64(PEM) — the shape Secrets and .env files inject.
#
# Usage:
#   scripts/gen-keys.sh                # print export lines to stdout
#   scripts/gen-keys.sh > keys.env     # capture into an env file (gitignored)
#   scripts/gen-keys.sh --k8s NS       # emit a k8s Secret manifest (namespace NS)
#
# Without keys the service falls back to HS256 using JWT_SECRET (fine for local
# dev / CI); RS256 is the production path.
set -euo pipefail

command -v openssl >/dev/null 2>&1 || { echo "openssl is required" >&2; exit 1; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$tmp/private.pem" 2>/dev/null
openssl pkey -in "$tmp/private.pem" -pubout -out "$tmp/public.pem" 2>/dev/null

# base64, single line (portable: strip newlines rather than rely on -w0).
b64() { base64 < "$1" | tr -d '\n'; }
PRIV_B64="$(b64 "$tmp/private.pem")"
PUB_B64="$(b64 "$tmp/public.pem")"

if [[ "${1:-}" == "--k8s" ]]; then
  ns="${2:-default}"
  cat <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: auth-service-keys
  namespace: ${ns}
type: Opaque
data:
  JWT_PRIVATE_KEY: ${PRIV_B64}
  JWT_PUBLIC_KEY: ${PUB_B64}
EOF
  exit 0
fi

cat <<EOF
# RS256 keypair for auth-service (base64(PEM), single line). Keep the private
# key secret; commit neither. Source this file or load into your Secret store.
export JWT_PRIVATE_KEY=${PRIV_B64}
export JWT_PUBLIC_KEY=${PUB_B64}
EOF
