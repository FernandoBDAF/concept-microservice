# Pattern: Kubernetes deploy shapes (kustomize) that survived v2→v5

The manifest layout, per-service-class defaults, network policy starter, and
CI drift check that the lab hardened across the v2 (kind), v3 (observability),
and v5 (AWS) phases. This is the "how we lay out k8s" pattern a copying project
can adopt whole, then trim.

Every value below was read from `deploy/k8s/` in the POST-v4 tree; treat them
as tuned starting points, not gospel. Citations are lab code paths and, for
the design lessons, experiment write-up filenames.

## Context

The goal is one source of truth per fact, a base that describes the whole
system, and thin overlays that encode only per-environment differences. The
lab runs the *same* base against local `kind` (single and multinode) and,
in v5, against EKS via an `aws` overlay — so the base has to be
environment-neutral and the overlays small.

## The layout

`deploy/k8s/` is a kustomize base with per-environment overlays:

```
deploy/k8s/
  base/
    kustomization.yaml       # whole-lab aggregator (resources: every component)
    namespaces/              # lab-core (services), lab-infra (stores/jobs), lab-obs
    infra/                   # postgres, redis, rabbitmq, mongodb, minio
    migrations/              # one-shot migrate Jobs + generated ConfigMaps
    services/                # api, auth, graphrag + 3 workers
    ingress/                 # ingress + TLS
    netpols/                 # lab-core.yaml, lab-infra.yaml
  overlays/
    kind-local/              # daily single-node — passthrough (resources: ../../base)
    kind-multinode/          # base + node-pinning patches
  vendor/                    # pinned cert-manager, ingress-nginx manifests
```

Overlays reference the base by relative path and add only deltas.
`kind-local` is a pure passthrough (`resources: [../../base]`).
`kind-multinode` adds `patches:` that inject
`nodeSelector: {node-type: infrastructure}` onto every store, so
local-path PVs (which carry node affinity) don't strand data when a node is
drained or killed — the drill that motivated it is
`documentation/experiments/2026-07-10-v2-cluster-exit-runs.md` (EXP-21).

### Single-source config: configMapGenerator + LoadRestrictionsNone

The lab refuses to fork config that already lives somewhere authoritative. It
does this with `configMapGenerator.files:` pointing at paths *above* the
kustomization root, single-sourcing them from their real home
(`deploy/k8s/base/*/kustomization.yaml`):

- RabbitMQ config is generated from the compose config dir
  (`scripts/compose/rabbitmq/…`) so cluster and compose share one file.
- Migration ConfigMaps are generated from each service's own `migrations/`
  dir (`api-service/migrations/*.up.sql`, …) so the SQL is never copied.

Kustomize's default load restrictor forbids reading files above the root, so
**every build must pass `--load-restrictor LoadRestrictionsNone`**:

```
kustomize build --load-restrictor LoadRestrictionsNone deploy/k8s/overlays/kind-local
```

`make cluster-up`, the drift check, and CI all pass the flag
(`deploy/k8s/README.md`). This is the deliberate trade: single-sourced config
in exchange for one non-default build flag everyone must remember.

## Per-service-class defaults

The lab has three shapes. Copy the one that matches, don't average them.

**Core HTTP services** (`api-service`) — `deploy/k8s/base/services/api-service/`:
- `replicas: 2`, RollingUpdate `maxSurge: 1 / maxUnavailable: 0`
- requests `cpu 150m / mem 128Mi`; limits `cpu 500m / mem 256Mi`
- PDB `minAvailable: 1`
- probes: `startupProbe /health` (period 2s, failureThreshold 30),
  `readinessProbe /ready` (period 5s, timeout 3s),
  `livenessProbe /health` (period 10s, timeout 5s)
- `preStop: sleep 5` (see drain note below)

**Single-replica stateful-ish service** (`auth-service`) —
`deploy/k8s/base/services/auth-service/`:
- `replicas: 1` (in-app lockout state is per-instance memory, ADR-009.5)
- requests `cpu 100m / mem 128Mi`; limits `cpu 300m / mem 256Mi`
- PDB **`maxUnavailable: 1`** — deliberately *not* `minAvailable`, so a
  1-replica deploy doesn't block node drains forever. This is the key nuance:
  `minAvailable: 1` on a 1-replica workload wedges every drain.
- `preStop: sleep 5`

**Workers** (`email-`/`image-`/`profile-worker`, identical) —
`deploy/k8s/base/services/*-worker/`:
- `replicas: 1`, label `lab.local/role: worker`, no rolling-update block
- requests `cpu 50m / mem 64Mi`; limits `cpu 300m / mem 128Mi`
- **no PDB** (they drain by reconnecting to the broker, not by staying up)

(`graphrag-service` is its own case: `replicas 1`, limits `300m / 256Mi`, no
PDB, and it deliberately omits a `runAsUser` override because forcing a
foreign UID breaks its pip `--user` layout — a lesson from EXP-20, below.)

**Stores** use `exec` probes and carry no PDB:
- Postgres: `exec pg_isready -U postgres`; limits `cpu 500m / mem 512Mi`
- Redis: `exec redis-cli ping`; limits `cpu 200m / mem 128Mi`

### The rolling-update drain seam (EXP-22)

`deploy/k8s/base/services/{api,auth}-service/deployment.yaml` add a
`lifecycle.preStop: exec sleep 5` on the two front-door services and nowhere
else. `documentation/experiments/2026-07-10-v2-cluster-exit-runs.md` (EXP-22)
found that without it a rolling update briefly routed new connections to a
terminating pod (2/200 failed probes) because endpoint propagation lags
SIGTERM; the 5s sleep holds the pod alive until endpoints drop, and a re-run
went 150/150. No explicit `terminationGracePeriodSeconds` is set — the pods
rely on the 30s default, which comfortably covers the 5s sleep.

## Network policy starter

`deploy/k8s/base/netpols/` holds **15 policies** across two namespaces
(`lab-core.yaml`: 6; `lab-infra.yaml`: 9). The shape is:

1. **`default-deny-all`** per namespace — `podSelector: {}`,
   `policyTypes: [Ingress, Egress]`, no rules. Denies everything.
2. **`allow-dns`** per namespace — `podSelector: {}` (selects *every* pod),
   egress UDP+TCP 53 to `kube-system`. DNS is opened for all pods separately.
3. **Explicit per-service allows**, e.g. api→auth egress to `app: auth-service`
   on `:3000`; the `postgres` store policy admits only api, auth, the migrate
   Jobs, and the exporter on `:5432`. Kubelet probes bypass NetworkPolicy by
   design, so probes keep working under default-deny.

### The union-of-allows lesson (EXP-23) — read this before editing netpols

`documentation/experiments/2026-07-10-v2-cluster-exit-runs.md` (EXP-23) proved
the denied paths deny and the allowed paths work — but opening one new path
(worker→postgres) needed **three** deletions, not one. The trap, quoted from
the write-up: **"`allow-dns`'s empty podSelector selects every pod, and any
selecting egress policy imposes default-deny on everything it doesn't allow.
Union-of-allows; no deny primitive."**

Practical consequences for a copier:

- NetworkPolicy has no deny rule. The effective policy for a pod is the
  *union* of every policy that selects it; a connection is allowed only if
  some policy explicitly allows it. To open a path you add an allow; to close
  one you remove every allow that grants it.
- Because `allow-dns` selects all pods with an egress rule, **every pod is
  already under egress default-deny** the moment netpols are applied — you
  cannot rely on "no egress policy = allow all egress."
- A path needs *both* sides: an egress allow on the client **and** an ingress
  allow on the server. Grep both `lab-core.yaml` and `lab-infra.yaml` when
  reasoning about a path.
- Re-applying the overlay restores the full set — treat the netpol dir as the
  single source, don't hand-edit live policies.

## Drift-check as a CI institution

The lab treats config drift as a first-class defect and greps for it in CI.

- `make drift-check` runs `python3 scripts/check-kustomize-drift.py`
  (`Makefile`), which treats `docker-compose.yml` as the behavioral reference
  and the kustomize render as its twin, comparing four surfaces: **service
  set** (every compose service maps to a workload), **env keys** (identical
  per-service key sets, with an explicit allowlist for secret-expansion
  helpers), **infra images** (exact pins; built services must reference the
  local registry), and **migration ConfigMaps** (every `*.sql` on disk is in
  the generated ConfigMap — catches a forgotten migration).
- It is wired into CI as a dedicated `drift-check` job in
  `.github/workflows/ci.yml` (runs on pushes to main and all PRs): Python 3.12,
  `pip install pyyaml`, kustomize v5.5.0, then the script.
- EXP-23's acceptance notes it is "proven two-sided": a deliberate env-key typo
  produced symmetric errors and exit 1, and reverting went green — so it
  catches drift in either direction, not just missing cluster keys.

The institution is the point: the same fact (a topology/env key) lives in
compose and in kustomize, and a bot fails the build the moment they disagree —
which is how the class of "manifests silently went stale" bug (named in
`documentation/review/CONCEPTUAL_REVIEW.md` §10) stops recurring.

## Adaptation checklist

- [ ] Keep the base = whole-system, overlays = env-deltas split. Start with a
      passthrough overlay for your dev environment.
- [ ] Single-source any config shared with another home
      (`configMapGenerator.files:` above-root) and standardize on
      `--load-restrictor LoadRestrictionsNone` in *every* build entry point
      (Makefile, drift check, CI).
- [ ] Pick a PDB per class deliberately: `minAvailable: 1` for multi-replica
      services, `maxUnavailable: 1` (or no PDB) for single-replica ones so
      drains don't wedge.
- [ ] Add `preStop: sleep 5` on anything behind a Service that takes rolling
      updates; rely on the 30s default grace period unless you need more.
- [ ] Ship netpols as default-deny + all-pods `allow-dns` + explicit
      per-path allows, and internalize union-of-allows: to open a path add
      egress *and* ingress; to close one remove every allow. Treat the netpol
      dir as the single source.
- [ ] Wire a drift check into CI from day one — compare your local-run config
      against your rendered manifests on every PR.
