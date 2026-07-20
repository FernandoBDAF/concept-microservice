#!/usr/bin/env bash
# make init-secrets (ADR-009.3): generate lab credentials once, apply them as
# k8s Secrets in both namespaces. Values persist in .lab-secrets.env
# (gitignored) so re-runs and cluster rebuilds reuse them; FORCE=1 rotates.
# Compose mode is untouched — it keeps its lab-default .env values.
#
# SKIP_POSTGRES=1 (AWS sessions, v5): don't touch any postgres-credentials
# Secret — on EKS those are owned by the external-secrets ExternalSecret
# (real RDS password from Secrets Manager); a random one here would fight the
# controller and be wrong anyway. rabbitmq/mongo/jwt stay seeded here on AWS
# too (follow-up: move them to Secrets Manager for uniformity).
set -euo pipefail
cd "$(dirname "$0")/../.."

SKIP_POSTGRES="${SKIP_POSTGRES:-0}"

ENVFILE=".lab-secrets.env"

if [ "${FORCE:-0}" = "1" ]; then rm -f "$ENVFILE"; fi

if [ ! -f "$ENVFILE" ]; then
  # RSA-2048 keypair for auth-service RS256 (ADR-009.1). Stored base64(PEM)
  # single-line so it fits the env-file model and matches the k8s Secret shape
  # (and the compose .env shape, scripts/compose/gen-jwt-keys.sh).
  keydir="$(mktemp -d)"
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$keydir/private.pem" 2>/dev/null
  openssl pkey -in "$keydir/private.pem" -pubout -out "$keydir/public.pem" 2>/dev/null
  JWT_PRIVATE_KEY_B64="$(base64 < "$keydir/private.pem" | tr -d '\n')"
  JWT_PUBLIC_KEY_B64="$(base64 < "$keydir/public.pem" | tr -d '\n')"
  rm -rf "$keydir"
  cat > "$ENVFILE" <<EOF
POSTGRES_PASSWORD=$(openssl rand -hex 16)
AUTH_DB_PASSWORD=$(openssl rand -hex 16)
RABBITMQ_PASSWORD=$(openssl rand -hex 16)
MONGO_ROOT_PASSWORD=$(openssl rand -hex 16)
MINIO_ROOT_PASSWORD=$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -hex 32)
JWT_PRIVATE_KEY=$JWT_PRIVATE_KEY_B64
JWT_PUBLIC_KEY=$JWT_PUBLIC_KEY_B64
SEED_ADMIN_EMAIL=admin@lab.local
SEED_ADMIN_PASSWORD=admin-$(openssl rand -hex 8)
EOF
  echo "generated $ENVFILE"
fi
# shellcheck disable=SC1090
. "./$ENVFILE"

for ns in lab-core lab-infra lab-obs; do
  kubectl create namespace "$ns" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
done

apply_secret() { # ns name key=value...
  local ns="$1" name="$2"; shift 2
  local args=()
  for kv in "$@"; do args+=(--from-literal="$kv"); done
  kubectl -n "$ns" create secret generic "$name" "${args[@]}" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  echo "  $ns/$name"
}

echo "applying secrets:"
for ns in lab-infra lab-core; do
  [ "$SKIP_POSTGRES" = "1" ] || apply_secret "$ns" postgres-credentials \
    "POSTGRES_PASSWORD=$POSTGRES_PASSWORD" "AUTH_DB_PASSWORD=$AUTH_DB_PASSWORD"
  # username stays `guest` — the api-service viper default (CONTRACTS.md §4);
  # the docker image permits remote guest, and the password is real anyway.
  # ⚠️ Guest-password caveat (ADR-008.4): the broker loads deploy/rabbitmq/
  # definitions.json at boot via load_definitions, which declares the guest user
  # from a password_hash baked into that committed file (currently guest/guest).
  # This rotated RABBITMQ_PASSWORD is NOT reflected there, so on kind the broker
  # keeps accepting "guest" until the definitions are regenerated with it:
  #   python3 scripts/rabbitmq/generate-definitions.py --password "$RABBITMQ_PASSWORD"
  # and the rabbitmq-config configMap rebuilt / STS restarted. Auto-wiring this
  # is deferred — see the v4 deferral ledger (guest-password ⇄ definitions.json).
  apply_secret "$ns" rabbitmq-credentials \
    "RABBITMQ_USER=guest" "RABBITMQ_PASSWORD=$RABBITMQ_PASSWORD"
  apply_secret "$ns" mongodb-credentials "MONGO_ROOT_PASSWORD=$MONGO_ROOT_PASSWORD"
  apply_secret "$ns" minio-credentials "MINIO_ROOT_PASSWORD=$MINIO_ROOT_PASSWORD"
done
# auth-service RS256 keypair (ADR-009.1) — its own Secret so key rotation does
# not churn the other auth credentials; base64(PEM) single-line (app decodes it)
apply_secret lab-core auth-service-keys \
  "JWT_PRIVATE_KEY=$JWT_PRIVATE_KEY" "JWT_PUBLIC_KEY=$JWT_PUBLIC_KEY"
# JWT_SECRET stays for the HS256 fallback + refresh signing; SEED_ADMIN_*
# bootstraps the admin role on first boot (ADR-009.7)
apply_secret lab-core auth-service-secrets \
  "JWT_SECRET=$JWT_SECRET" \
  "SEED_ADMIN_EMAIL=$SEED_ADMIN_EMAIL" "SEED_ADMIN_PASSWORD=$SEED_ADMIN_PASSWORD"
# lab-obs: the postgres-exporter (deploy/obs, ADR-003.5) reads the postgres
# password from this Secret in its own namespace. Skipped on AWS with the
# others — the exporter needs an ExternalSecret + RDS host there (follow-up,
# registered in v5-HANDOFF).
[ "$SKIP_POSTGRES" = "1" ] || apply_secret lab-obs postgres-credentials \
  "POSTGRES_PASSWORD=$POSTGRES_PASSWORD" "AUTH_DB_PASSWORD=$AUTH_DB_PASSWORD"
