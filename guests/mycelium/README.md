# mycelium (KM)

A GraphRAG YouTube→knowledge-graph pipeline onboarded as a lab **guest** per
[`documentation/HOST_CONTRACT.md`](../../documentation/HOST_CONTRACT.md).
Recon + architecture: [`documentation/deployment/KM_DEPLOYMENT_PLAN.md`](../../documentation/deployment/KM_DEPLOYMENT_PLAN.md).
This is the **lab-side** onboarding scaffolding (phase v7, KM-3): compose
project, k8s namespace, netpols, ServiceMonitors, dashboard and `launch.yaml`.

## Components

| Component    | What it is                                          | Container port | Compose host |
|--------------|-----------------------------------------------------|----------------|--------------|
| `stages-api` | FastAPI/uvicorn (stages bind); **/health /metrics** | 8080 (`http`)  | 4220         |
| `graph-api`  | Same FastAPI app (graph bind)                       | 8081 (`http`)  | 4221         |
| `stages-ui`  | Next.js 16 (home route `/explore`)                  | 3000 (`http`)  | 4210         |

The two "APIs" are **one consolidated FastAPI/uvicorn process** that dual-binds
`:8080` (stages) and `:8081` (graph) —
`GraphRAG/src/app/api/fastapi_app/main.py` (`create_app` + `_run_dual_bind`).
The legacy standalone stdlib `http.server` entrypoints still exist as secondary
launchers but are no longer the primary surface. **GraphDash was retired in
phase 4** (no `:3001`, no directory); StagesUI is the only front-end.

Port block **42xx** (HOST_CONTRACT §1.4 registry). Pipelines run **in-process,
sequentially**, over a shared `PipelineRunner`; stage handoff is via **MongoDB
collections**:

- ingestion: `ingest→clean→chunk→enrich→embed→redundancy→trust`
- graphrag: `graph_extraction→entity_resolution→graph_construction→community_detection→insights_generation`

Whole pipeline **runs** are dispatched through an **arq/Redis run-queue**
(`GraphRAG/src/app/workers/`, phase-3), falling back to an in-process thread if
Redis is absent. RabbitMQ / `km.stage.<name>` stage-level queues are the lab's
**future KM-4 queue-mode target** (ADR-007.4), *not* current mycelium behavior.

## How it maps to shared infra (ADR-007.1)

Opt-in shared infra on the lab's instances. mycelium's real stores today:

- **MongoDB** (source of truth) — stage collections; production uses Atlas
  Vector Search (dim 1024). Lab db `mycelium` on `mongodb.lab-infra:27017`.
- **Neo4j** — secondary, rebuildable graph read-store (graceful-degrades if
  absent).
- **Redis** — the arq run-queue (phase-3). No Postgres, no MinIO/S3.

The **RabbitMQ vhost `mycelium`** (`rabbitmq.lab-infra:5672`) with stage queues
`km.stage.<name>`, retry tiers 5s/30s/2m and DLQ `km.stage.<name>.dlq`
(ADR-007.4) is the lab's **future KM-4 queue-mode topology** — *not* a current
mycelium dependency. Today mycelium has no RabbitMQ/amqp code at all.

## Fake-mode spend-proof (ADR-007.3, EXP-70)

The spend-proof is *by absence*: the k8s NetworkPolicies
([`k8s/base/netpols.yaml`](k8s/base/netpols.yaml)) open **no egress to
api.openai.com or api.voyageai.com**. A pod that cannot route to the LLM
providers cannot spend, regardless of what keys leak into its env. **That
netpol-level guarantee holds today** and is the real spend-proof for EXP-70.

The `LLM_MODE=fake` drill mode it complements is **unbuilt — the KM-1 spec, not
current behavior**: mycelium has no `LLM_MODE` var and no fake mode. It calls
real OpenAI (`GRAPHRAG_MODEL=gpt-4o-mini`; raises if `OPENAI_API_KEY` is unset)
and Voyage embeddings (`VOYAGE_RPM=20` is the throughput bottleneck). A
`dry_run` flag exists but only skips DB *writes* — it still spends. KM-1's spec
is deterministic fixtures keyed by input hash **plus a hard-fail if a real key
is set**.

## Guest-side pending — images are NOT built yet

**Honest status:** the mycelium repo **is present**
(`/home/fbarroso/forest/mycelium`, git `main`), but the lab guest **images**
(`localhost:5001/mycelium-<component>:dev`) don't exist yet — mycelium ships no
Dockerfiles and its CI builds no images. Guest-side PRs must land first:

- **KM-1** — fake-LLM mode (`LLM_MODE=fake`, committed fixtures). *Greenfield —
  no fake mode exists today.*
- **KM-2** — Dockerfiles + a `/ready` handler. `/health`
  (`fastapi_app/main.py:179`) and `/metrics` (`:155`, Prometheus text)
  **already exist upstream**; `/ready` is the real gap. Metric names are
  unprefixed (`chunks_processed`, `stage_duration_seconds`) — no `mycelium_*`.

Until KM-1/KM-2 land, `make guest-up G=mycelium` (and the compose/kind `up`
targets) will **fail to pull images** — this scaffolding is statically valid
(kustomize/compose/JSON all parse) but not yet runnable.

## Contract evidence

| Contract clause (HOST_CONTRACT.md) | Evidence here |
|---|---|
| §1.2 /health /ready /metrics | `/health` (`fastapi_app/main.py:179`) + `/metrics` (`:155`) **already exist upstream**; `/ready` is the gap (KM-2). Probes wired in `k8s/base/<component>.yaml`; `/metrics` scraped via `k8s/obs` |
| §1.3 `launch.yaml` | [`launch.yaml`](launch.yaml) — compose + kind up/down/status, components, ports |
| §1.4 assigned port block | 42xx: stages-ui 4210, stages-api 4220, graph-api 4221 |
| §1.6 deployment note | KM_DEPLOYMENT_PLAN.md "Production target shape" + this README |
| §2 isolation (host side) | compose project `mycelium` + own default network; namespace `mycelium` (`lab.local/tier: guest`), default-deny netpols |
| §2 shared infra (ADR-007.1) | Mongo db `mycelium` (+ Neo4j, Redis) — egress holes in `k8s/base/netpols.yaml`; RabbitMQ vhost is the future KM-4 target, not a live dep |
| §2 observability (host side) | compose: `mycelium-*` aliases on `microservices_default`; k8s: `k8s/obs/` ServiceMonitors + `k8s/dashboards/mycelium.json` |
| §2 ingress (host side) | `k8s/base/ingress.yaml` — `mycelium.lab.local`, TLS via `lab-ca` |

## Deployment note (contract §1.6)

For real, mycelium deploys on **shared infra** (ADR-007.1 opt-in): managed
Mongo with Atlas Vector Search (dim 1024), the consolidated FastAPI app and
StagesUI as Deployments behind one ingress, Neo4j for the graph read-store, and
the arq/Redis run-queue. Lab mode vs production mode differ in: registry
(`localhost:5001` → real), LLM (the **unbuilt** `LLM_MODE=fake` drill → real
OpenAI/Voyage keys, budgeted per EXP-71), Mongo (lab Mongo → Atlas with vector
search), and hostname. What it still lacks for production: fake-LLM mode itself
(KM-1) and images (KM-2) are unbuilt; stage idempotency needs auditing before
the future queue-mode (KM-4) retries are safe; no `/ready` and no alerting rules
on its metrics yet; single replica per component in the lab.
