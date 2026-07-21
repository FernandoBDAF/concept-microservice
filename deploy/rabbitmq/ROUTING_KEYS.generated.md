# Routing keys & topology (GENERATED — do not edit)

Source of truth: `scripts/rabbitmq/generate-definitions.py` →
`deploy/rabbitmq/definitions.json` (ADR-008.4). Services verify
(passively declare-check) but never author topology.

| routing key | exchange | work queue | retry tiers | DLQ (TTL) | prefetch | consumer |
|---|---|---|---|---|---|---|
| `profile.task` | `profile-tasks` | `profile-processing` | 5s/30s/2m | `profile-processing.dlq` (24h) | 2 | profile-worker |
| `email.send` | `email-tasks` | `email-processing` | 5s/30s/2m | `email-processing.dlq` (24h) | 5 | email-worker |
| `image.process` | `image-tasks` | `image-processing` | 5s/30s/2m | `image-processing.dlq` (72h) | 1 | image-worker |
| `document.process` | `document-tasks` | `document-processing` | 5s/30s/2m | `document-processing.dlq` (168h) | 1 | graphrag-service |

- `email-processing` staleness TTL 60min → `email-expired` (rk `email.expired`), never the poison DLQ (ADR-008.5).
- Retry flow (ADR-008.1): consumer publishes to `<exchange>.retry` rk `<rk>.retry.<tier>`, acks; wait-queue TTL dead-letters back to the main exchange. After the last tier, consumer publishes to `<exchange>.dlx` rk `<rk>` (poison) and acks.
- `task-results` (rk `task.result`): workers/graphrag publish completion/failure; api-service consumes (ADR-008.3).
- No `default-tasks` fallback — unknown routing keys are a publisher bug and fail fast (ADR-008.6).

## Guest vhost: `mycelium` (ADR-007.1/.4)

Exchange `mycelium-stages` (+ `.retry`/`.dlx`) on vhost `mycelium`.
Each pipeline stage is a work queue with the same 5s/30s/2m retry ladder
and a per-stage DLQ; `mycelium-results` (rk `km.stage.result`) closes the
loop. Built by the same generator so the guest inherits lab conventions —
the migration is the exercise (KM_DEPLOYMENT_PLAN §2).

| routing key | work queue | retry tiers | DLQ (TTL) |
|---|---|---|---|
| `km.stage.ingest` | `km.stage.ingest` | 5s/30s/2m | `km.stage.ingest.dlq` (168h) |
| `km.stage.clean` | `km.stage.clean` | 5s/30s/2m | `km.stage.clean.dlq` (168h) |
| `km.stage.enrich` | `km.stage.enrich` | 5s/30s/2m | `km.stage.enrich.dlq` (168h) |
| `km.stage.chunk` | `km.stage.chunk` | 5s/30s/2m | `km.stage.chunk.dlq` (168h) |
| `km.stage.embed` | `km.stage.embed` | 5s/30s/2m | `km.stage.embed.dlq` (168h) |
| `km.stage.redundancy` | `km.stage.redundancy` | 5s/30s/2m | `km.stage.redundancy.dlq` (168h) |
| `km.stage.trust` | `km.stage.trust` | 5s/30s/2m | `km.stage.trust.dlq` (168h) |
| `km.stage.graph_extraction` | `km.stage.graph_extraction` | 5s/30s/2m | `km.stage.graph_extraction.dlq` (168h) |
| `km.stage.entity_resolution` | `km.stage.entity_resolution` | 5s/30s/2m | `km.stage.entity_resolution.dlq` (168h) |
| `km.stage.graph_construction` | `km.stage.graph_construction` | 5s/30s/2m | `km.stage.graph_construction.dlq` (168h) |
| `km.stage.community_detection` | `km.stage.community_detection` | 5s/30s/2m | `km.stage.community_detection.dlq` (168h) |

Totals: 17 exchanges, 78 queues, 78 bindings (across all vhosts).
