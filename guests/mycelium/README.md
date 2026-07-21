# mycelium (KM)

A GraphRAG YouTube→knowledge-graph pipeline onboarded as a lab **guest** per
[`documentation/HOST_CONTRACT.md`](../../documentation/HOST_CONTRACT.md).
Recon + architecture: [`documentation/deployment/KM_DEPLOYMENT_PLAN.md`](../../documentation/deployment/KM_DEPLOYMENT_PLAN.md).
This is the **lab-side** onboarding scaffolding (phase v7, KM-3): compose
project, k8s namespace, netpols, ServiceMonitors, dashboard and `launch.yaml`.

## Components

| Component    | What it is                              | Container port | Compose host |
|--------------|-----------------------------------------|----------------|--------------|
| `stages-api` | Python stdlib `http.server`; **/metrics** | 8080 (`http`)  | 4220         |
| `graph-api`  | Python stdlib `http.server`             | 8080 (`http`)  | 4221         |
| `stages-ui`  | Next.js                                 | 3000 (`http`)  | 4210         |
| `graphdash`  | Next.js                                 | 3000 (`http`)  | 4211         |

Port block **42xx** (HOST_CONTRACT §1.4 registry). The pipeline runs stages
(`ingest→clean→enrich→chunk→embed→redundancy→trust`, then
`graph_extraction→entity_resolution→graph_construction→community_detection`);
stage handoff is via MongoDB collections today, and via `km.stage.<name>`
lab-envelope messages in queue-mode (KM-4, guest-side).

## How it maps to shared infra (ADR-007.1)

Opt-in shared infra on the lab's instances — no standalone stores:

- **MongoDB database `mycelium`** on `mongodb.lab-infra:27017` — stage
  collections + the graph store (lab rehearsal uses no Atlas Vector Search;
  fake mode supplies deterministic pseudo-vectors, dim 1024).
- **RabbitMQ vhost `mycelium`** on `rabbitmq.lab-infra:5672` — stage queues
  `km.stage.<name>` with retry tiers 5s/30s/2m and DLQ `km.stage.<name>.dlq`
  (topology owned by the orchestrator's generator; ADR-007.4). No
  Postgres/MinIO needs.

## Fake-mode spend-proof (ADR-007.3, EXP-70)

Lab drills run `LLM_MODE=fake`: LLM (OpenAI) and embeddings (Voyage) return
deterministic fixtures/pseudo-vectors — **zero real keys, zero spend**. The
proof is *by absence*: the k8s NetworkPolicies
([`k8s/base/netpols.yaml`](k8s/base/netpols.yaml)) open egress to Mongo and
RabbitMQ only — **there is deliberately no egress rule to api.openai.com or
api.voyageai.com**. A pod that cannot route to the LLM providers cannot spend,
regardless of what keys leak into its env. That netpol-level guarantee is the
spend-proof for EXP-70; fake mode also hard-fails guest-side if a real key is
SET (KM-1).

## Guest-side pending — images are NOT built yet

**Honest status:** the container images
(`localhost:5001/mycelium-<component>:dev`) are built from the **mycelium
repo** (`~/repo/forest/mycelium`), which is **not yet onboarded**. The
guest-side PRs must land first:

- **KM-1** — fake-LLM mode (`LLM_MODE=fake`, committed fixtures).
- **KM-2** — Dockerfiles + trivial `/health` and `/ready` handlers on the two
  stdlib APIs (stages-api already exposes `/metrics`).

Until KM-1/KM-2 land, `make guest-up G=mycelium` (and the compose/kind `up`
targets) will **fail to pull images** — this scaffolding is statically valid
(kustomize/compose/JSON all parse) but not yet runnable. The `/health`,
`/ready` probes and the exact metric names in the dashboard are **confirmed
guest-side**; comments in the manifests mark each such assumption.

## Contract evidence

| Contract clause (HOST_CONTRACT.md) | Evidence here |
|---|---|
| §1.2 /health /ready /metrics | probes on `/health` + `/ready` in each `k8s/base/<component>.yaml`; stages-api `/metrics` scraped via `k8s/obs` (handlers added guest-side, KM-2) |
| §1.3 `launch.yaml` | [`launch.yaml`](launch.yaml) — compose + kind up/down/status, components, ports |
| §1.4 assigned port block | 42xx: stages-ui 4210, graphdash 4211, stages-api 4220, graph-api 4221 |
| §1.6 deployment note | KM_DEPLOYMENT_PLAN.md "Production target shape" + this README |
| §2 isolation (host side) | compose project `mycelium` + own default network; namespace `mycelium` (`lab.local/tier: guest`), default-deny netpols |
| §2 shared infra (ADR-007.1) | Mongo db `mycelium`, RabbitMQ vhost `mycelium` — egress holes in `k8s/base/netpols.yaml` |
| §2 observability (host side) | compose: `mycelium-*` aliases on `microservices_default`; k8s: `k8s/obs/` ServiceMonitors + `k8s/dashboards/mycelium.json` |
| §2 ingress (host side) | `k8s/base/ingress.yaml` — `mycelium.lab.local`, TLS via `lab-ca` |

## Deployment note (contract §1.6)

For real, mycelium deploys on **shared infra** (ADR-007.1 opt-in): managed
Mongo with vector search (Atlas serverless to start), the two APIs and two UIs
as Deployments behind one ingress, a pipeline worker consuming stage tasks.
Lab mode vs production mode differ in: registry (`localhost:5001` → real),
LLM (`LLM_MODE=fake` → real OpenAI/Voyage keys, budgeted per EXP-71), Mongo
(lab Mongo, no vector search → Atlas), and hostname. What it still lacks for
production: real vector-store dependency for `embed`→`redundancy` is an open
question (KM_DEPLOYMENT_PLAN), stage idempotency needs auditing before
queue-mode retries are safe, no alerting rules on its metrics yet, single
replica per component in the lab.
