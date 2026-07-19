# Host contract — guest onboarding (v0, DRAFT)

> Status: **v0 draft** (phase v3). Implements ADR-007.1 (shared infra by
> default) and ADR-007.2 (namespace-per-guest + documented port blocks).
> Graduates to **v1 in phase v7**, once mycelium and loam have onboarded
> against it and the rough edges are known. Reference guest:
> [`guests/hello-guest/`](../guests/hello-guest/) — every clause below is
> proven there (see its README for the clause → evidence table).

A **guest** is an external system that runs inside the lab to inherit its
platform: orchestration, observability, isolation, ingress. This document is
the deal. Every MUST is checkable by a command; a guest that fails a check is
not onboarded.

## 1. What a guest MUST provide

### 1.1 Containerized components, pinned images
Every component runs as a container built from a Dockerfile in the guest's
tree. Base images are pinned (tag at minimum, e.g. `golang:1.24-alpine`,
`alpine:3.19` — never bare `latest`). Images are tagged
`localhost:5001/<guest>-<component>:dev` for the lab registry.

*Check:* `docker build` succeeds from the guest dir; `grep -c ':latest' Dockerfile` → 0.

### 1.2 Health, readiness, metrics endpoints
Every **long-running** component serves over HTTP:

| Endpoint   | Meaning                                          | Check |
|------------|--------------------------------------------------|-------|
| `/health`  | process is alive (liveness)                      | `curl -fs …/health` → 200 |
| `/ready`   | dependencies reachable, can take traffic         | `curl -fs …/ready` → 200 |
| `/metrics` | Prometheus text format, exposition version 0.0.4 | `curl -fs …/metrics \| grep '^# TYPE'` |

Metric names are prefixed `<guest>_<component>_` (underscored) and MUST be
stable — the shared dashboards query them by name. One-shot jobs (migrations,
init containers) are exempt from all three.

### 1.3 Launch definition: `launch.yaml` at the guest root
Machine-readable entry point so host tooling (Makefile targets, later Mission
Control) can drive any guest uniformly. Schema (v0):

```yaml
name: <guest>            # required — matches dir, namespace, compose project
description: <one line>  # required
port_block: "41xx"       # required — from the registry in §1.4
targets:                 # required — shell commands, run from the repo root
  compose: {up: …, down: …, status: …}
  kind:    {up: …, down: …, status: …}
links:                   # optional — [{name, url}] humans can open
components:              # required — one entry per long-running component
  - name: <component>
    ports:               # [{name, container, compose_host}]
      - {name: http, container: 8080, compose_host: 4100}
    metrics_path: /metrics
```

*Check:* file exists, parses as YAML, and each `targets.*.status` command runs.

### 1.4 An assigned port block (compose host ports)
Compose publishes guest ports only inside the guest's assigned hundred-block.
The registry (this document is the source of truth — claim a block by PR):

| Block | Guest            |
|-------|------------------|
| 41xx  | hello-guest      |
| 42xx  | mycelium (KM)    |
| 43xx  | loam             |

*Check:* every `ports:` host side in the guest's compose file is inside the block.

### 1.5 At least one guest experiment
`<guest>/EXPERIMENTS.md` with ≥1 drill in the lab's format (Goal / Validates /
Steps / Expect / Cleanup) whose assertions are runnable commands. The
experiment must exercise the guest **through the host's observability** — a
drill nobody can watch proves nothing.

### 1.6 A deployment note
A short section (README or `DEPLOYMENT.md`) answering: how would this deploy
for real — standalone or on shared infra (the ADR-007.1 declaration), what
changes between lab mode and production mode, and what the guest still lacks
for production. Honesty over polish; "not production-ready because X, Y" is a
passing answer.

## 2. What the host provides

- **Shared infra, opt-in (ADR-007.1):** on request, a dedicated postgres
  database / mongo database / minio bucket / rabbitmq vhost on the lab's
  shared instances — named `<guest>` or `<guest>_<purpose>`. Guests whose
  production plan is standalone may BYO instead; declare it in the deployment
  note.
- **Isolation (ADR-007.2):** compose — own project (guest-local
  `docker-compose.yml` with `name: <guest>`), own default network; k8s — own
  namespace labeled `lab.local/tier: guest` + `lab.local/guest: <guest>`,
  default-deny NetworkPolicies. Guests are invisible to each other and to
  lab-core unless a policy says otherwise.
- **Observability:** Prometheus scrapes every declared `/metrics` endpoint
  (compose: static targets on the `microservices_default` network; k8s:
  ServiceMonitors in the guest's `k8s/obs/` overlay, applied when the obs
  stack is installed). Guest metrics are queryable in the shared Grafana;
  logs flow into the shared log pipeline with the rest of the cluster.
- **Ingress (optional):** `<guest>.lab.local` through the shared
  ingress-nginx, TLS via the `lab-ca` ClusterIssuer.

## 3. Guest tree layout (convention, not contract)

```
guests/<guest>/
  launch.yaml           # §1.3
  docker-compose.yml    # own project name; attaches to microservices_default
  Dockerfile(s)
  k8s/base/             # namespace, workloads, netpols, ingress — no CRDs
  k8s/obs/              # ServiceMonitors etc. — needs the obs stack's CRDs
  EXPERIMENTS.md        # §1.5
  README.md             # tour + contract evidence + deployment note (§1.6)
```

`k8s/base` MUST apply cleanly on a bare kind cluster (no CRDs); anything
requiring the obs stack lives in `k8s/obs`.

## 4. Open questions for v1 (phase v7)

- Resource quotas per guest namespace (none enforced in v0).
- Secret injection pattern for guests needing credentials (ADR-007.6 covers
  loam's agents; generalize?).
- `launch.yaml` schema versioning and validation tooling.
- Whether the port-block registry moves out of prose into a checked file.
