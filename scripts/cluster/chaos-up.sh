#!/usr/bin/env bash
# make chaos-up: install Chaos Mesh (ADR-004.3) into the chaos-mesh namespace
# via Helm (pinned). kind runs containerd, so chaosDaemon must point at kind's
# containerd socket. Idempotent: helm upgrade --install converges an existing
# install. Chaos CRs live in deploy/chaos/ and are applied per-experiment
# (kubectl apply -f), not here — the runner drives them as cli steps.
set -euo pipefail
cd "$(dirname "$0")/../.."

# chart pin: latest stable, verified 2026-07-19 via Artifact Hub + GitHub
# releases (chaos-mesh/chaos-mesh v2.8.3, released 2026-06-10; chart version ==
# app version). Bump deliberately.
CHAOS_MESH_VERSION=2.8.3

step() { printf '\n\033[1;34m== %s\033[0m\n' "$*"; }

step "1/3 helm repo"
helm repo add chaos-mesh https://charts.chaos-mesh.org >/dev/null 2>&1 || true
helm repo update chaos-mesh >/dev/null

step "2/3 namespace chaos-mesh"
kubectl create namespace chaos-mesh --dry-run=client -o yaml | kubectl apply -f - >/dev/null

step "3/3 install chaos-mesh ${CHAOS_MESH_VERSION}"
# kind's node containerd socket (not the host docker socket) — the chaosDaemon
# joins pod network/pid namespaces through it to inject faults.
helm upgrade --install chaos-mesh chaos-mesh/chaos-mesh \
  --version "$CHAOS_MESH_VERSION" -n chaos-mesh \
  --set chaosDaemon.runtime=containerd \
  --set chaosDaemon.socketPath=/run/containerd/containerd.sock \
  --wait --timeout 5m

printf '\n\033[1;32mchaos-mesh ready\033[0m\n'
cat <<'EOF'
Apply an experiment CR, then delete it to recover:
  kubectl apply  -f deploy/chaos/podchaos-kill-worker.yaml
  kubectl apply  -f deploy/chaos/networkchaos-api-postgres-200ms.yaml
  kubectl delete -f deploy/chaos/<file>.yaml
Dashboard: kubectl -n chaos-mesh port-forward svc/chaos-dashboard 2333:2333
EOF
