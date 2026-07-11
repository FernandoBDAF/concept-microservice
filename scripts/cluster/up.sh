#!/usr/bin/env bash
# make cluster-up [PROFILE=single|multinode]: kind cluster + local registry
# (ADR-002.3) + ingress-nginx + cert-manager (vendored, pinned) + secrets +
# images + the kustomize overlay, then wait until everything is Ready.
# Idempotent: safe to re-run on an existing cluster to converge it.
set -euo pipefail
cd "$(dirname "$0")/../.."

PROFILE="${PROFILE:-single}"
CLUSTER=lab
REG_NAME=kind-registry
REG_PORT=5001
OVERLAY="deploy/k8s/overlays/kind-local"
[ "$PROFILE" = "multinode" ] && OVERLAY="deploy/k8s/overlays/kind-multinode"

step() { printf '\n\033[1;34m== %s\033[0m\n' "$*"; }

step "1/8 local registry localhost:${REG_PORT}"
if [ "$(docker inspect -f '{{.State.Running}}' $REG_NAME 2>/dev/null)" != "true" ]; then
  docker run -d --restart=always -p "127.0.0.1:${REG_PORT}:5000" \
    --name $REG_NAME registry:2 >/dev/null
fi

step "2/8 kind cluster '$CLUSTER' ($PROFILE)"
if ! kind get clusters 2>/dev/null | grep -qx $CLUSTER; then
  kind create cluster --config "deploy/kind/${PROFILE}.yaml"
else
  echo "   cluster exists — converging"
fi

step "3/8 wire registry into containerd + network"
REGISTRY_DIR="/etc/containerd/certs.d/localhost:${REG_PORT}"
for node in $(kind get nodes --name $CLUSTER); do
  docker exec "$node" mkdir -p "$REGISTRY_DIR"
  cat <<EOF | docker exec -i "$node" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
[host."http://${REG_NAME}:5000"]
EOF
done
docker network connect kind $REG_NAME 2>/dev/null || true
cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REG_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

step "4/8 ingress-nginx (vendored)"
kubectl apply -f deploy/k8s/vendor/ingress-nginx-kind-v1.11.2.yaml >/dev/null
# rate-limit rejections should say 429, not the nginx default 503 (ADR-009.5)
kubectl -n ingress-nginx patch configmap ingress-nginx-controller \
  --type merge -p '{"data":{"limit-req-status-code":"429"}}' >/dev/null
kubectl -n ingress-nginx wait deploy/ingress-nginx-controller \
  --for=condition=Available --timeout=180s

step "5/8 cert-manager (vendored)"
kubectl apply -f deploy/k8s/vendor/cert-manager-v1.15.3.yaml >/dev/null
kubectl -n cert-manager wait deploy --all --for=condition=Available --timeout=180s

step "6/8 secrets (ADR-009.3)"
bash scripts/cluster/init-secrets.sh

step "7/8 images -> localhost:${REG_PORT}"
make --no-print-directory images

step "8/8 apply ${OVERLAY} and wait"
# Jobs are immutable; drop finished/stale ones so changed migrations re-run
kubectl -n lab-infra delete job api-migrate auth-migrate minio-bucket-init \
  --ignore-not-found >/dev/null
# cert-manager's webhook can lag its Available condition; retry the apply
for i in $(seq 1 12); do
  if kustomize build --load-restrictor LoadRestrictionsNone "$OVERLAY" \
     | kubectl apply -f - >/dev/null 2>/tmp/lab-apply-err; then
    break
  fi
  [ "$i" = 12 ] && { cat /tmp/lab-apply-err; exit 1; }
  echo "   apply retry $i (webhook warming up)"; sleep 5
done

echo "   waiting for infra rollouts"
for sts in postgres rabbitmq mongodb minio; do
  kubectl -n lab-infra rollout status statefulset/$sts --timeout=300s
done
kubectl -n lab-infra rollout status deploy/redis --timeout=120s
echo "   waiting for one-shot jobs"
kubectl -n lab-infra wait job --all --for=condition=Complete --timeout=300s
echo "   waiting for service rollouts"
for d in api-service auth-service graphrag-service email-worker image-worker profile-worker; do
  kubectl -n lab-core rollout status deploy/$d --timeout=300s
done
kubectl -n lab-core wait certificate --all --for=condition=Ready --timeout=120s

printf '\n\033[1;32mcluster ready\033[0m\n'
grep -q 'api.lab.local' /etc/hosts 2>/dev/null || cat <<'EOF'

hosts file: api.lab.local / auth.lab.local are not in /etc/hosts. Either:
  sudo sh -c 'echo "127.0.0.1 api.lab.local auth.lab.local" >> /etc/hosts'
or use curl --resolve api.lab.local:443:127.0.0.1 (no sudo; what CI/EXP
write-ups use). Details: deploy/k8s/README.md.
EOF
make --no-print-directory cluster-status
