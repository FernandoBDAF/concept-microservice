#!/usr/bin/env bash
# make init-secrets (ADR-009.3): generate lab credentials once, apply them as
# k8s Secrets in both namespaces. Values persist in .lab-secrets.env
# (gitignored) so re-runs and cluster rebuilds reuse them; FORCE=1 rotates.
# Compose mode is untouched — it keeps its lab-default .env values.
set -euo pipefail
cd "$(dirname "$0")/../.."

ENVFILE=".lab-secrets.env"

if [ "${FORCE:-0}" = "1" ]; then rm -f "$ENVFILE"; fi

if [ ! -f "$ENVFILE" ]; then
  cat > "$ENVFILE" <<EOF
POSTGRES_PASSWORD=$(openssl rand -hex 16)
AUTH_DB_PASSWORD=$(openssl rand -hex 16)
RABBITMQ_PASSWORD=$(openssl rand -hex 16)
MONGO_ROOT_PASSWORD=$(openssl rand -hex 16)
MINIO_ROOT_PASSWORD=$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -hex 32)
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
  apply_secret "$ns" postgres-credentials \
    "POSTGRES_PASSWORD=$POSTGRES_PASSWORD" "AUTH_DB_PASSWORD=$AUTH_DB_PASSWORD"
  # username stays `guest` — the api-service viper default (CONTRACTS.md §4);
  # the docker image permits remote guest, and the password is real anyway
  apply_secret "$ns" rabbitmq-credentials \
    "RABBITMQ_USER=guest" "RABBITMQ_PASSWORD=$RABBITMQ_PASSWORD"
  apply_secret "$ns" mongodb-credentials "MONGO_ROOT_PASSWORD=$MONGO_ROOT_PASSWORD"
  apply_secret "$ns" minio-credentials "MINIO_ROOT_PASSWORD=$MINIO_ROOT_PASSWORD"
done
apply_secret lab-core auth-service-secrets "JWT_SECRET=$JWT_SECRET"
