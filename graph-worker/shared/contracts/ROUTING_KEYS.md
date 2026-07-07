# Routing Keys

Canonical RabbitMQ topology. Publisher (api-service `DefaultRoutingMap`) and
every consumer MUST declare byte-identical arguments — RabbitMQ rejects a
redeclare with mismatched args (`PRECONDITION_FAILED`), and a queue only
receives what is published to the exchange it is bound to.

## Exchanges and Queues

| Routing Key | Exchange | Queue | Message TTL | DLQ TTL | Max retries | Prefetch | Worker |
|-------------|----------|-------|-----|---------|-------------|----------|--------|
| `document.process` | `document-tasks` | `document-processing` | 12h | 7d | 3 | 1 | graphrag-service |
| `email.send` | `email-tasks` | `email-processing` | 1h | 24h | 5 | 5 | email-worker |
| `image.process` | `image-tasks` | `image-processing` | 6h | 3d | 2 | 1 | image-worker |
| `profile.task` | `profile-tasks` | `profile-processing` | 1h | 24h | 3 | 2 | profile-worker |

## Dead-letter convention

Every exchange `X` has a direct durable DLX `X.dlx`; every queue `Q` has a DLQ
`Q.dlq` bound to the DLX. Main queues carry `x-dead-letter-exchange`,
`x-dead-letter-routing-key`, `x-message-ttl`, and `x-max-retries`; DLQs carry
their own `x-message-ttl` (the "DLQ TTL" column). Consumers nack poison
messages without requeue so they land in the DLQ instead of redelivering
forever.

## Notes
- Exchange types are **direct**; exchanges and queues are durable.
- Prefetch is consumer-side QoS (not a declare arg), tuned to worker processing time.
- Unknown routing keys published via api-service fall back to `default-tasks` /
  `default-processing` (a parking lot with no consumer).
- Message envelope and payload schemas: [MESSAGE_FORMAT.md](MESSAGE_FORMAT.md).
