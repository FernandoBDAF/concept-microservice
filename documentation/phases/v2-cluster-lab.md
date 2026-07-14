# Phase v2 — Restore the cluster lab (kind)

**Status:** ✅ shipped 2026-07-10 (same session as the v1.1 exit run) ·
**Exit tag:** `lab-v2.0` · **Exit runs:** EXP-20..23 all pass — see
[the run report](../experiments/2026-07-10-v2-cluster-exit-runs.md); three
defects fixed en route (mongodb probe YAML, rabbitmq probe economics,
runAsNonRoot vs named users), two design findings recorded
(readiness-coupling → CONCEPTUAL_REVIEW §12; netpol union-of-allows →
EXP-23's calibrated text) · **Decisions in force:** ADR-002 (all),
ADR-009.4 (cert-manager), ADR-010.2 (CI phase 1), ADR-010.4 (legacy mining)

## Mission

`make cluster-up` runs the **entire v1 stack on kind**, and every v1
experiment works against the cluster. This restores the k8s practice
substrate the 2026-01 consolidation archived — rebuilt around the
consolidated architecture instead of resurrected.

## Context a fresh session needs

- The compose stack (root `docker-compose.yml`) is the behavioral reference:
  13 containers — postgres (api_db+auth_db init), redis, rabbitmq (plugins +
  per-object metrics conf), mongodb, minio (+bucket init), api-migrate/
  auth-migrate one-shots, api-service (8080/8081), auth-service (3000),
  graphrag-service, 3 workers, prometheus, grafana. Env contract per service:
  `CONTRACTS.md` §4.
- Per-service k8s manifests already exist (`api-service/deployments/
  kubernetes/`, worker/graphrag equivalents) — stale but a starting point.
- **Era-1 material to mine (ADR-010.4):** `legacy_project/k8s/` and git
  `1efec36:k8s/` — kind-config.yaml + kind-multinode.yaml, default-deny
  network policies, ingress-nginx/metrics-server/storage-class setups,
  numbered per-service deployments with probes/limits/security contexts,
  setup scripts. Mine patterns, not files — the service set changed.

## Work breakdown

1. **Layout:** `deploy/k8s/base/` kustomize base per component + overlays
   `deploy/k8s/overlays/kind-local/` (ADR-002.1). Namespaces: `lab-core`
   (services), `lab-infra` (data stores), `lab-obs` (reserved for v3).
2. **kind profiles (ADR-002.2):** `deploy/kind/single.yaml` +
   `multinode.yaml` (1 control-plane + 3 workers, port mappings for ingress);
   mine era-1 configs.
3. **Local registry (ADR-002.3):** registry container at `localhost:5001`
   wired into kind (containerd mirror config); `make images` builds+tags+
   pushes all 6 images; manifests reference `localhost:5001/...`.
4. **Infra in-cluster:** StatefulSets (postgres w/ init SQL ConfigMap, mongo,
   minio, rabbitmq w/ enabled_plugins+conf mounts) + Deployments (redis);
   one-shot migrations as Jobs mirroring compose's api-migrate/auth-migrate;
   minio bucket init Job.
5. **Services:** Deployments for api/auth/graphrag/3 workers with the
   compose-equivalent env (Secrets for passwords via `make init-secrets`,
   ADR-009.3), liveness/readiness probes on the real endpoints, resource
   requests/limits (start from era-1's numbers), PDBs for api/auth.
6. **Ingress + TLS:** ingress-nginx; hostnames `api.lab.local`,
   `auth.lab.local`, (grafana/rabbitmq later or port-forward); cert-manager
   with a lab CA issuing ingress certs (ADR-009.4); document the /etc/hosts
   or *.localtest.me choice.
7. **Network policies:** default-deny per namespace + explicit allows
   (api→infra, workers→rabbitmq+their needs, auth→postgres, ingress→api/auth)
   — port era-1's zero-trust model to the new shape.
8. **Rate limiting at ingress (ADR-009.5):** nginx annotations on auth
   routes; app limiter stays for compose mode.
9. **Make targets:** `cluster-up` (kind create + registry + apply overlays +
   wait healthy), `cluster-down`, `cluster-status`, `images`,
   `cluster-logs S=`; experiments' underlying levers get cluster-aware
   variants where needed (worker-outage via `kubectl scale --replicas=0`,
   etc.).
10. **CI phase 1 (ADR-010.2):** GitHub Actions on push: `make verify`, image
    builds, and the compose⇄kustomize drift check (ADR-002.4 — script
    comparing images+env keys between compose config and kustomize build).
11. **Legacy archive (ADR-010.4):** after mining, move `legacy_project/` to
    branch `archive/era-1`, delete from main.

## Out of scope

Observability in-cluster (v3 — the compose Grafana/Prometheus keep serving
until then; cluster experiments that need metrics can port-forward the
compose pair or wait), OpenTelemetry, hello-guest (v3), any messaging/auth
architecture changes (v4).

## Exit experiments (extend EXPERIMENTS.md)

- **EXP-20 — Catalog parity on kind:** EXP-01..09 + EXP-11 pass against the
  cluster (EXP-10/12 need only trivial command swaps). Document the command
  mapping (docker compose ⇄ kubectl).
- **EXP-21 — Node kill & rescheduling (multinode):** under `sim-load`, drain/
  delete a worker node; watch pods reschedule, PDBs hold, load recover;
  write the analysis (era-1 k6-analysis quality bar).
- **EXP-22 — Image path:** change one service, `make images`, rolling restart
  picks the new digest from localhost:5001; rollback via previous tag.
- **EXP-23 — Zero-trust proof:** from a scratch pod in lab-core, show denied
  paths are denied (e.g. worker→postgres) and allowed paths work; netpol
  removal makes the denied path work (then restore).

## Acceptance

- [x] `make cluster-up` from clean machine state → all pods Ready < ~10 min
      (multinode from deleted cluster: 9 m 21 s incl. builds; single is
      faster)
- [x] EXP-20..23 pass, written up in documentation/experiments/
      (2026-07-10-v2-cluster-exit-runs.md)
- [x] CI phase 1 workflow added (`make verify` green locally); drift check
      proven red on deliberate env drift, green after revert. *CI-on-main
      turns green with the merge/push of this branch.*
- [x] legacy_project archived to branch `archive/era-1`; main slimmed
- [x] Tag `lab-v2.0`
