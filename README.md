# Microservices — Consolidated Profile & Document Platform

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

## Monitoring & simulations

The point of this lab is practicing operations: generate load or failures with
one command, watch the system react in Grafana. With the stack up:

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
├── documentation/            # PRD, reviews, deployment, guides, planning archive
└── legacy_project/           # archived pre-consolidation code (era-1 k8s lab)
```

## Deployment

Local development uses the root docker-compose. Kubernetes manifests live per
service (`api-service/deployments/kubernetes/`, etc.); the target cluster
architecture is described in
[documentation/deployment/CLUSTER_VISION.md](documentation/deployment/CLUSTER_VISION.md),
and the roadmap to a full kind-based cluster lab is
[documentation/PRD.md](documentation/PRD.md) (v2 milestone).

---

**Architecture:** consolidated service (v2, refactored 2026-07) ·
**Contracts:** [CONTRACTS.md](CONTRACTS.md) ·
**Docs:** [documentation/](documentation/) ·
**PRD:** [documentation/PRD.md](documentation/PRD.md)
