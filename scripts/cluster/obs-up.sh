#!/usr/bin/env bash
# make obs-up: the observability stack (ADR-003) into lab-obs —
# kube-prometheus-stack + Tempo + postgres/redis exporters (Helm, pinned)
# then deploy/obs/manifests (ServiceMonitors, alerts, OpenSearch+fluent-bit,
# ntfy+relay, ingresses, netpols). Idempotent: helm upgrade --install +
# kubectl apply converge an existing install. Assumes the cluster from
# `make cluster-up` (CRD apply ordering: charts first, manifests after).
set -euo pipefail
cd "$(dirname "$0")/../.."

# OBS_LOGS off skips the log pipeline (OpenSearch + fluent-bit + index Job) —
# AWS sessions opt out per drill for RAM/cost (v5 HANDOFF §7). Default on:
# kind behavior is unchanged.
OBS_LOGS="${OBS_LOGS:-1}"
# Treat 0/false/no/off (any case, or empty) as OFF; anything else as ON. The
# old `!= "0"` test wrongly enabled logs for OBS_LOGS=false. obs-down.sh uses
# the same semantics.
obs_logs_on() {
  case "$(printf '%s' "$OBS_LOGS" | tr '[:upper:]' '[:lower:]')" in
    0 | false | no | off | "") return 1 ;;
    *) return 0 ;;
  esac
}

# chart pins (helm search verified 2026-07-19; bump deliberately)
KPS_VERSION=87.17.0
TEMPO_VERSION=1.24.4
PG_EXPORTER_VERSION=8.2.0
REDIS_EXPORTER_VERSION=6.27.0

step() { printf '\n\033[1;34m== %s\033[0m\n' "$*"; }

step "1/6 helm repos"
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
helm repo add grafana https://grafana.github.io/helm-charts >/dev/null 2>&1 || true
helm repo update prometheus-community grafana >/dev/null

step "2/6 secrets (postgres-exporter DSN)"
# postgres-exporter reads Secret postgres-credentials in lab-obs —
# init-secrets.sh must include lab-obs in its namespace loop (v3 wiring).
# On AWS (SKIP_POSTGRES=1) that Secret is deliberately not seeded — the
# exporter needs an ExternalSecret + RDS host there (v5 follow-up), so warn
# instead of dying.
bash scripts/cluster/init-secrets.sh
kubectl -n lab-obs get secret postgres-credentials >/dev/null 2>&1 || {
  if [ "${SKIP_POSTGRES:-0}" = "1" ]; then
    echo "WARN: lab-obs/postgres-credentials absent (SKIP_POSTGRES=1) —"
    echo "postgres-exporter will be down until its ExternalSecret lands (v5 follow-up)."
  else
    echo "ERROR: lab-obs/postgres-credentials missing — extend"
    echo "scripts/cluster/init-secrets.sh to apply it into lab-obs."
    exit 1
  fi
}

step "3/6 kube-prometheus-stack ${KPS_VERSION}"
helm upgrade --install kps prometheus-community/kube-prometheus-stack \
  --version "$KPS_VERSION" -n lab-obs \
  -f deploy/obs/kube-prometheus-stack-values.yaml \
  --wait --timeout 10m

step "4/6 tempo ${TEMPO_VERSION} + exporters"
helm upgrade --install tempo grafana/tempo \
  --version "$TEMPO_VERSION" -n lab-obs \
  -f deploy/obs/tempo-values.yaml --wait --timeout 5m
helm upgrade --install postgres-exporter prometheus-community/prometheus-postgres-exporter \
  --version "$PG_EXPORTER_VERSION" -n lab-obs \
  -f deploy/obs/postgres-exporter-values.yaml --wait --timeout 5m
helm upgrade --install redis-exporter prometheus-community/prometheus-redis-exporter \
  --version "$REDIS_EXPORTER_VERSION" -n lab-obs \
  -f deploy/obs/redis-exporter-values.yaml --wait --timeout 5m

step "5/6 apply deploy/obs/manifests"
kustomize build --load-restrictor LoadRestrictionsNone deploy/obs/manifests \
  | kubectl apply -f -
if obs_logs_on; then
  # Jobs are immutable; drop the finished/stale one so template changes re-run
  kubectl -n lab-obs delete job lab-logs-index-template --ignore-not-found >/dev/null
  kustomize build deploy/obs/manifests/logs | kubectl apply -f -
else
  echo "OBS_LOGS=$OBS_LOGS — skipping OpenSearch/fluent-bit (deploy/obs/manifests/logs)"
fi

step "6/6 wait for rollouts"
if obs_logs_on; then
  kubectl -n lab-obs rollout status statefulset/opensearch --timeout=300s
  kubectl -n lab-obs rollout status deploy/opensearch-dashboards --timeout=300s
  kubectl -n lab-obs rollout status daemonset/fluent-bit --timeout=180s
fi
kubectl -n lab-obs rollout status deploy/ntfy --timeout=120s
# ntfy-relay needs localhost:5001/ntfy-relay:dev — built by `make images`
kubectl -n lab-obs rollout status deploy/ntfy-relay --timeout=180s
obs_logs_on && kubectl -n lab-obs wait job/lab-logs-index-template --for=condition=Complete --timeout=300s
# cert-manager exists on kind only (EKS terminates TLS at the ALB with ACM) —
# wait on Certificates only where the CRD is installed
if kubectl get crd certificates.cert-manager.io >/dev/null 2>&1; then
  kubectl -n lab-obs wait certificate --all --for=condition=Ready --timeout=120s
fi

printf '\n\033[1;32mobs stack ready\033[0m\n'
cat <<'EOF'
UIs (via /etc/hosts or curl --resolve, same as api.lab.local):
  https://grafana.lab.local        admin/admin (Lab Overview + k8s dashboards)
  https://prometheus.lab.local     targets, rules
  https://alertmanager.lab.local   routing, silences
  https://opensearch.lab.local     Discover on lab-logs-*
  https://ntfy.lab.local           topic: lab-alerts
Docs: documentation/deployment/OBSERVABILITY.md
EOF
