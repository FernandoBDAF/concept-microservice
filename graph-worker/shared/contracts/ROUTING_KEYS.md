<!-- GENERATED — do not edit here. Edit scripts/rabbitmq/generate-definitions.py,
     then run `make routing-keys`. Source of truth: deploy/rabbitmq/definitions.json. -->

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

Totals: 13 exchanges, 22 queues, 22 bindings.
