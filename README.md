# coppice — Consolidated Profile & Document Platform

**coppice** (formerly concept-microservice) — forest floor in the forest-layer
naming: the ops lab; hosts guest systems, cut back and regrown on purpose.

A profile and document-processing platform built around one consolidated Go API
service with direct infrastructure access, an independent Node.js auth service,
and asynchronous workers behind RabbitMQ.

## Architecture

```
                        ┌──────────────┐  POST /v1/auth/token/validate
          ┌────────────▶│ auth-service │◀──────────────┐
          │             │ (Node/TS)    │               │ JWT validation
          ▼             └──────┬───────┘               │ (only inter-service HTTP)
       Client                  │ postgres (auth_db)    │
          │             ┌──────▼───────────────────────┴──────┐
          └────────────▶│           api-service (Go/Gin)      │
                        │  profiles · documents · tasks       │
                        └──┬──────────┬──────────┬─────────┬──┘
                    direct │   direct │   direct │  direct │
                        ┌──▼───┐ ┌────▼─────┐ ┌──▼─────┐ ┌─▼────┐
                        │Redis │ │PostgreSQL│ │RabbitMQ│ │MinIO │
                        │cache │ │ (api_db) │ └──┬─────┘ │ docs │
                        └──────┘ └──────────┘    │       └──────┘
                 ┌───────────────┬───────────────┼──────────────┐
                 ▼               ▼               ▼              ▼
          email-worker    image-worker    profile-worker  graphrag-service
             (Go)            (Go)            (Go)         (Python → MongoDB)
```

- **api-service** talks to Postgres/Redis/RabbitMQ/MinIO directly (no internal
  HTTP hops); its only HTTP dependency is token validation against auth-service.
- **Workers** consume the four task queues; **graphrag-service** builds a
  knowledge graph in MongoDB from uploaded documents.
- Every cross-service surface (queues, message envelope, auth contract, env
  vars, ports) is pinned in **[CONTRACTS.md](CONTRACTS.md)**.

## Projects

| Project | Stack | Port(s) | Purpose |
|---|---|---|---|
| [api-service](api-service/) | Go 1.24 · Gin · sqlx · go-redis v9 · amqp091 · MinIO | 8080 (API), 8081 (metrics) | Profiles, documents, task submission |
| [auth-service](auth-service/) | Node 22 · TypeScript · Express · pg · zod | 3000 | Register/login/refresh + token validation |
| [graph-worker/operational-workers](graph-worker/operational-workers/) | Go 1.24 | — | email / image / profile queue consumers |
| [graph-worker/graphrag-service](graph-worker/graphrag-service/) | Python 3.12 · aio-pika · MongoDB | 8082→8080 (health) | `document.process` consumer, knowledge graph |

Infrastructure: PostgreSQL 15 (`api_db` + `auth_db`), Redis 7, RabbitMQ 3.12
(management UI :15672, Prometheus :15692), MongoDB 7 (host port 27018), MinIO
(console :9001). Observability: **Prometheus :9090** and **Grafana :3001**
(admin/admin) with a provisioned *Lab Overview* dashboard.

## Quick start

```bash
cp .env.example .env          # optional: adjust JWT secret etc.

make up                       # full stack: infra + migrations + all services
# or:
make infra                    # infra + DB migrations + bucket only
cd api-service && make run    # then run services locally against it

make ps                       # status
make logs S=api-service       # tail one service
make down                     # stop (make nuke also deletes volumes)
```

Verify every project builds and passes tests (local toolchains: Go 1.24+,
Node 22+, Python 3.12+):

```bash
make verify
```

## Experiments, monitoring & simulations

The point of this lab is practicing operations: generate load or failures with
one command, watch the system react in Grafana. **Start with
[EXPERIMENTS.md](EXPERIMENTS.md)** — a guided catalog (baseline, load, burst,
poison, outages, scale-out, incident reproduction, TTL expiry, document E2E)
that tells you what to run, what to watch, and what to expect; running it top
to bottom validates the whole implementation. Write findings to
[documentation/experiments/](documentation/experiments/).

The underlying levers, with the stack up:

```bash
make monitoring     # print the UI URLs (Grafana 3001, Prometheus 9090, RabbitMQ 15672)
make queues         # RabbitMQ queue depths / consumers at a glance

make sim-smoke      # k6: 1 VU sanity pass (auth → API → queues → workers)
make sim-load       # k6: steady load     (override: SIM_VUS=20 SIM_DURATION=5m)
make sim-burst      # k6: 50-VU burst
make sim-poison     # malformed messages to every exchange → watch DLQ panel
make sim-outage     # stop a worker, flood its queue, restart, watch it drain
                    #   (override: WORKER=image N=200)
```

The scenario catalog grows in [documentation/PRD.md](documentation/PRD.md) (v4).

## API surface (api-service, `/api/v1`, Bearer JWT required)

```
GET  /health · GET /ready · GET /metrics          # public, per service

GET/POST      /api/v1/profiles                    # list (paginated) / create
GET/PUT/DELETE /api/v1/profiles/:id
POST          /api/v1/profiles/:id/tasks          # generic task submit
POST          /api/v1/profiles/:id/tasks/email    # → email.send
POST          /api/v1/profiles/:id/tasks/image    # → image.process
POST          /api/v1/profiles/:id/tasks/profile  # → profile.task
GET           /api/v1/profiles/:id/documents

POST          /api/v1/documents/upload            # → MinIO + document.process
GET           /api/v1/documents/:id[/status|/download]
DELETE        /api/v1/documents/:id
```

Auth endpoints (auth-service, port 3000): see [auth-service/README.md](auth-service/README.md).

## Async task contract

| Routing key | Exchange | Queue | Consumer |
|---|---|---|---|
| `document.process` | `document-tasks` | `document-processing` | graphrag-service |
| `email.send` | `email-tasks` | `email-processing` | email-worker |
| `image.process` | `image-tasks` | `image-processing` | image-worker |
| `profile.task` | `profile-tasks` | `profile-processing` | profile-worker |

Envelope and payload schemas: [graph-worker/shared/contracts](graph-worker/shared/contracts/).

## Repository layout

```
├── docker-compose.yml        # full local stack incl. Prometheus + Grafana
├── Makefile                  # up/down/logs/verify + monitoring & sim-* drills
├── EXPERIMENTS.md            # guided experiment catalog (run this!)
├── CONTRACTS.md              # pinned cross-service integration surface
├── scripts/
│   ├── compose/              # postgres init, prometheus config, grafana provisioning
│   └── simulate/             # k6 load script, queue publisher, outage drill
├── api-service/              # Go API (cmd, internal, migrations, k8s manifests)
├── auth-service/             # TypeScript auth (src, migrations)
├── graph-worker/
│   ├── operational-workers/  # Go consumers (email/image/profile)
│   ├── graphrag-service/     # Python document consumer
│   └── shared/contracts/     # routing keys + message format (canonical)
├── deploy/                   # kind profiles + kustomize tree (cluster lab, PRD v2)
└── documentation/            # PRD, reviews, deployment, guides, planning archive
```

Era-1 (the pre-consolidation k8s lab) lives on branch `archive/era-1`
(mined into `deploy/` during v2 — ADR-010.4).

## Deployment

Local development uses the root docker-compose (the behavioral reference).
The same stack runs on Kubernetes via the kind cluster lab (PRD v2):

```
make cluster-up                 # kind + local registry + ingress/TLS + full stack
make cluster-status             # nodes, pods, jobs, certificates
make cluster-sim-smoke          # k6 through https://api.lab.local
make cluster-queues             # rabbitmqctl inside the cluster
make cluster-down               # delete the cluster (registry cache survives)
make cluster-up PROFILE=multinode   # 1 control-plane + 3 workers (node drills)
```

Manifests: [deploy/k8s/](deploy/k8s/README.md) (kustomize base + overlays,
zero-trust network policies, cert-manager lab CA, localhost:5001 registry).
`make drift-check` keeps compose and kustomize declaring the same surface.
Cluster experiments: EXPERIMENTS.md EXP-20..23.

---

**Architecture:** consolidated service (v2, refactored 2026-07) ·
**Contracts:** [CONTRACTS.md](CONTRACTS.md) ·
**Docs:** [documentation/](documentation/) ·
**PRD:** [documentation/PRD.md](documentation/PRD.md)
