# mycelium (KM) deployment plan — v7 draft

**Repo:** `~/repo/forest/mycelium` (phase doc's `~/repo/mycelium` is stale).
Recon 2026-07-19; plan precedes onboarding (ADR-007), rehearsed in the lab
before any real deployment.

## What it actually is (recon summary)

Three projects: `GraphRAG/` (Python 3.12; two stdlib-`http.server` APIs —
Stages :8080, Graph :8081 — plus CLI ingest), `StagesUI/` (Next 16, :3000),
`GraphDash/` (Next 15, :3001). Pipelines run **in-process, sequentially**
(`PipelineRunner`): ingestion `ingest→clean→enrich→chunk→embed→redundancy→
trust`, then graphrag `graph_extraction→entity_resolution→
graph_construction→community_detection`. Stage handoff = **MongoDB
collections** (no queues today). Store = **Mongo only** (vector dim 1024 ⇒
Atlas Vector Search assumed). LLM = OpenAI (`src/lib/llm/client.py`,
`GRAPHRAG_MODEL=gpt-4o-mini`); embeddings = Voyage (`VOYAGE_RPM=20` — the
throughput bottleneck); `YOUTUBE_API_KEY` for transcripts. **No app
Dockerfiles.** Metrics exist (Prometheus via Stages API `/metrics`);
tracing stubbed; structured logging present. Concurrency env-tuned
(prod ~300 per graphrag stage; CPU + network-I/O profile, no GPU).

## Production target shape

- **Compute:** one pipeline-runner deployment per pipeline invocation model
  — start with a single "pipeline worker" consuming stage tasks (see
  migration below), scaled horizontally per stage class later; two API
  deployments (stages/graph) + two UI deployments behind one ingress.
- **Stores:** managed Mongo w/ vector search (Atlas serverless to start).
  Lab rehearsal: the lab's Mongo (no vector search) + `EMBEDDER=fake` in
  fake mode; a real-key session can point at a free-tier Atlas cluster.
- **Cost model:** dominated by LLM spend (OpenAI per-token + Voyage 20 RPM
  cap); infra is small (CPU pods + Mongo). Fake mode ⇒ $0 LLM. Budgeted
  real run (EXP-71): cap via `MAX_ITERATIONS`/batch sizes; record actual $.
- **Scaling knobs:** `*_CONCURRENCY`, `*_BATCH_SIZE`, `VOYAGE_RPM`,
  `MAX_ITERATIONS` (dev 5–10 concurrency vs prod 300 — the lab drills use
  dev numbers).

## Lab rehearsal scope (what v7 proves)

1. Containerization (guest-side): Dockerfiles for GraphRAG (one image, two
   API entrypoints + CLI) and the two UIs; `guests/mycelium/` compose +
   k8s per HOST_CONTRACT (port block 42xx: 4210 StagesUI, 4211 GraphDash,
   4220 stages-api, 4221 graph-api).
2. **Queue-conventions migration (ADR-007.4 — the exercise):** stage
   *boundaries* become lab-envelope messages on the mycelium vhost.
   Minimal honest version: `PipelineRunner` gains a `--queue-mode` where
   stage completion publishes `km.stage.<name>` (lab envelope; payload =
   {video_id/batch ref, source_collection, dest_collection}) and a single
   consumer advances the next stage — retry tiers + DLQ per lab topology.
   In-process mode stays default upstream; queue-mode is the lab rehearsal.
3. **Fake-LLM mode (ADR-007.3 — contract prerequisite):** deterministic
   stub keyed by input hash — spec in v7-HANDOFF §KM-1. Zero real keys
   mounted in fake mode (EXP-70 proves by absence).
4. Shared-infra mapping (ADR-007.1): Mongo database `mycelium` on the
   lab's Mongo; RabbitMQ vhost `mycelium`; no Postgres/MinIO needs.
5. Observability: its Prometheus metrics scraped by the lab stack
   (ServiceMonitor); its Loki/Promtail stack NOT deployed (lab OpenSearch
   ships logs); tracing stays stubbed (out of scope).

## Open questions (answer during onboarding)

- Atlas Vector Search dependency: does `embed`→`redundancy` hard-require
  vector indexes, or can fake mode skip similarity? (Determines whether
  fake mode also needs a fake vector store.)
- Stage idempotency: are collection writes upsert-shaped (safe under
  redelivery) or append-only? Audit before queue-mode.
- StagesUI assumes same-origin API (`NEXT_PUBLIC_STAGES_API_URL`) —
  ingress path routing vs subdomains.
