#!/usr/bin/env bash
# make chaos-down: remove Chaos Mesh (ADR-004.3) from the chaos-mesh namespace.
# Deletes any lingering experiment CRs first so finalizers don't strand the
# uninstall, then uninstalls the release. Idempotent — safe on a cluster that
# never had it. CRDs stay (helm does not remove them); drop by hand with
# `kubectl delete crd -l app.kubernetes.io/part-of=chaos-mesh` for a bare cluster.
set -euo pipefail
cd "$(dirname "$0")/../.."

step() { printf '\n\033[1;34m== %s\033[0m\n' "$*"; }

step "1/2 delete lingering chaos CRs"
kubectl delete -f deploy/chaos/ --ignore-not-found 2>/dev/null || true

step "2/2 uninstall chaos-mesh"
helm uninstall chaos-mesh -n chaos-mesh --wait 2>/dev/null \
  && echo "  uninstalled chaos-mesh" \
  || echo "  chaos-mesh not installed"

printf '\n\033[1;32mchaos-mesh removed\033[0m\n'
