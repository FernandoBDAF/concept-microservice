# mycelium (KM) deployment plan — v7 draft

**Repo:** `/home/fbarroso/forest/mycelium` — present as a sibling under the
shared `forest/` root (the phase doc's `~/repo/mycelium` and the earlier
`~/repo/forest/mycelium` paths are both stale). Re-reconned 2026-07-20
against `main` @06d2651; plan precedes onboarding (ADR-007), rehearsed in
the lab before any real deployment. The guest repo is present and was
re-reconned — the earlier v7 assumption that it was absent/un-startable is
void; guest-side work is startable, and what remains is called out as such
below.

## What it actually is (recon summary)

Two projects: `GraphRAG/` (Python 3.12) and `StagesUI/` (Next 16, :3000,
home route `/explore`). GraphRAG's **primary API surface is now a
consolidated FastAPI/uvicorn app** that binds **both :8080 (stages) and
:8081 (graph) in one process** (`GraphRAG/src/app/api/fastapi_app/main.py`,
`_run_dual_bind` ~L266–278). The old standalone stdlib-`http.server` APIs
(Stages :8080, Graph :8081) still exist as **secondary/legacy entrypoints**,
not the primary surface; CLI ingest remains. **`GraphDash/` was retired in
phase 4** — no `:3001`, no directory (only a pre-retirement `.git.bak`); any
doc citing GraphDash / Next 15 / :3001 is stale. Pipelines run **in-process,
sequentially** (`PipelineRunner`): ingestion
`ingest→clean→chunk→enrich→embed→redundancy→trust` — chunk **before** enrich
(`chunk_documents` reads `raw_documents` and must precede enrich/embed) —
then graphrag
`graph_extraction→entity_resolution→graph_construction→community_detection→insights_generation`
(`insights_generation` is mycelium's own phase-7 addition). Stage handoff =
**MongoDB collections**. Run dispatch now uses a **real arq/Redis job
queue** (phase 3, `GraphRAG/src/app/workers/`) that enqueues whole pipeline
**runs**, degrading gracefully to an in-process thread when Redis is absent
— so "no queues today" is stale, but note this is arq/Redis at the **run**
level, **not** RabbitMQ and **not** stage-level handoff. Stores =
**MongoDB** (source of truth; vector dim 1024 ⇒ Atlas Vector Search,
`$vectorSearch` in retrieval) **+ Neo4j** (secondary, rebuildable graph
read-store that graceful-degrades) **+ Redis** (arq); no Postgres, no
MinIO/S3. LLM = OpenAI (client `src/lib/llm/client.py`, raises if `OPENAI_API_KEY`
unset; `GRAPHRAG_MODEL` default `gpt-4o-mini` at
`src/core/config/graphrag.py:29`); embeddings = Voyage (embedder
`src/domain/ingestion/stages/embed.py`; `VOYAGE_RPM=20` — the throughput
bottleneck — read at `src/infrastructure/llm/rate_limit.py:15`);
`YOUTUBE_API_KEY` for transcripts; Anthropic only via optional AWS Bedrock
(not a hard dependency). **No app Dockerfiles anywhere** (CI
builds no images). `/health` **and** `/metrics` already exist on the
consolidated app (`main.py:179`, `:155`); `/ready` does **not** (a real
gap). Prometheus metric names are **unprefixed** — `chunks_processed` and
`stage_duration_seconds` (`src/lib/metrics/`), with no `mycelium_*` prefix.
Tracing stubbed; structured logging present. Concurrency env-tuned
(prod ~300 per graphrag stage; CPU + network-I/O profile, no GPU).

## Production target shape

- **Compute:** the arq worker consuming whole-run jobs off Redis
  (`src/app/workers/`), scaled horizontally later; **one consolidated API
  deployment** (the FastAPI app dual-binding 8080/8081) + **one UI
  deployment** (StagesUI) behind one ingress. Per-stage workers are a
  future queue-mode target, not today's shape — see migration below.
- **Stores:** managed Mongo w/ vector search (Atlas serverless to start) +
  Neo4j (secondary, rebuildable — graceful-degrades if absent) + Redis
  (arq run-queue). Lab rehearsal: the lab's Mongo + Neo4j + Redis; a
  real-key session can point at a free-tier Atlas cluster for
  `$vectorSearch`. **There is no fake / `EMBEDDER=fake` mode today** — it is
  greenfield (KM-1, §3 below); the `dry_run` flag that *does* exist only
  skips DB **writes** and still spends on OpenAI/Voyage.
- **Cost model:** dominated by LLM spend (OpenAI per-token + Voyage 20 RPM
  cap); infra is small (CPU pods + Mongo/Neo4j/Redis). A "fake mode ⇒ $0
  LLM" is the **KM-1 target spec, not current behavior** — nothing skips
  spend today. Budgeted real run (EXP-71): cap via `MAX_ITERATIONS`/batch
  sizes; record actual $.
- **Scaling knobs:** `*_CONCURRENCY`, `*_BATCH_SIZE`, `VOYAGE_RPM`,
  `MAX_ITERATIONS` (dev 5–10 concurrency vs prod 300 — the lab drills use
  dev numbers).

## Lab rehearsal scope (what v7 proves)

1. Containerization (guest-side, still TODO — **KM-2 greenfield; no
   Dockerfiles exist upstream**): a Dockerfile for GraphRAG (one image —
   the consolidated FastAPI app dual-binds 8080/8081 + CLI) and one for
   StagesUI. The lab's `guests/mycelium/` compose + k8s is **provisional
   scaffolding** standing in until real guest images exist (port block 42xx:
   4210 StagesUI, 4220 stages-api, 4221 graph-api). The retired-GraphDash
   slot (4211) and the wrong graph-api container port have been **removed /
   corrected in the lab manifests in this pass** (graph-api now :8081, the
   `graphdash` Deployment/service/ingress/dashboard-panel deleted).
2. **Queue-conventions migration (ADR-007.4 — the exercise, future KM-4):**
   stage *boundaries* become lab-envelope messages on the mycelium RabbitMQ
   vhost. **None of this exists in mycelium today** — no
   RabbitMQ/amqp/envelope/vhost/`km.stage` anywhere in the code; the
   existing arq/Redis queue is **run-level**, a different layer.
   Minimal honest version: `PipelineRunner` gains a `--queue-mode` where
   stage completion publishes `km.stage.<name>` (lab envelope; payload =
   {video_id/batch ref, source_collection, dest_collection}) and a single
   consumer advances the next stage — retry tiers + DLQ per lab topology.
   In-process (and run-level arq) mode stays default upstream; stage-level
   queue-mode is the lab rehearsal.
3. **Fake-LLM mode (ADR-007.3 — contract prerequisite, future KM-1):**
   deterministic stub keyed by input hash — spec in v7-HANDOFF §KM-1.
   **Unbuilt today: there is no fake mode and no `LLM_MODE` / `EMBEDDER=fake`
   env anywhere in mycelium** (the only `dry_run` flag just skips DB writes,
   still spends on OpenAI/Voyage). Zero real keys mounted in fake mode
   (EXP-70 proves by absence).
4. Shared-infra mapping (ADR-007.1): Mongo database `mycelium` **+ Neo4j +
   Redis (arq)** on the lab's shared infra; RabbitMQ vhost `mycelium` is the
   **future queue-mode target** (item 2), unused by mycelium today; no
   Postgres/MinIO needs.
5. Observability: its Prometheus metrics scraped by the lab stack
   (ServiceMonitor) — note the metric names are **unprefixed**
   (`chunks_processed`, `stage_duration_seconds`), so scrape config and
   dashboards must not assume a `mycelium_*` prefix; its Loki/Promtail stack
   NOT deployed (lab OpenSearch ships logs); tracing stays stubbed (out of
   scope).

## Open questions (answer during onboarding)

- Atlas Vector Search dependency: recon confirms it is **real** —
  `VECTOR_DIM=1024` (`core/config/paths.py:113`) and `$vectorSearch` in
  retrieval (`domain/rag/services/retrieval.py`), so the
  retrieval/redundancy path hard-requires Atlas Vector Search today. Open
  sub-question deferred to KM-1: can the (unbuilt) fake mode skip
  similarity, or does it also need a fake vector store?
- Stage idempotency: recon finds the ingestion writes are **upsert-shaped**
  (`update_one(..., upsert=True)` in `ingest_documents.py`, `ingest.py`,
  `backfill_transcript.py`) — safe under redelivery. Still audit the
  graphrag stages the same way before stage-level queue-mode; run-level
  redelivery is already exercised by the arq queue.
- StagesUI assumes same-origin API (`NEXT_PUBLIC_STAGES_API_URL`, with
  `NEXT_PUBLIC_GRAPH_API_URL` for graph) — ingress path routing vs
  subdomains. Simpler now that there is one UI and one consolidated API.
