#!/usr/bin/env bash
# make obs-down: remove the observability stack (ADR-003) from lab-obs.
# Idempotent — safe on a cluster that never had it. The namespace itself
# belongs to deploy/k8s/base (namespaces.yaml) and stays. The prometheus-
# operator CRDs also stay: helm does not uninstall CRDs, and dropping them
# would cascade-delete any monitors a re-install would want back; remove by
# hand with `kubectl delete crd -l app.kubernetes.io/part-of=kube-prometheus`
# if you really want a bare cluster.
set -euo pipefail
cd "$(dirname "$0")/../.."

step() { printf '\n\033[1;34m== %s\033[0m\n' "$*"; }

# Delete the UNION of everything obs-up can apply, regardless of the OBS_LOGS
# setting a previous obs-up ran with — the log pipeline lives in its own
# manifests dir (deploy/obs/manifests/logs: OpenSearch StatefulSet+PVC,
# dashboards, fluent-bit, index-template Job) and must be torn down too.
# --ignore-not-found makes each delete tolerant of a never-applied state.

step "1/3 delete deploy/obs/manifests/logs (log pipeline)"
# built without --load-restrictor, mirroring obs-up's logs apply
kustomize build deploy/obs/manifests/logs \
  | kubectl delete -f - --ignore-not-found

step "2/3 delete deploy/obs/manifests"
kustomize build --load-restrictor LoadRestrictionsNone deploy/obs/manifests \
  | kubectl delete -f - --ignore-not-found

step "3/3 uninstall helm releases"
for release in redis-exporter postgres-exporter tempo kps; do
  helm uninstall "$release" -n lab-obs --wait 2>/dev/null \
    && echo "  uninstalled $release" \
    || echo "  $release not installed"
done

printf '\n\033[1;32mobs stack removed\033[0m\n'
