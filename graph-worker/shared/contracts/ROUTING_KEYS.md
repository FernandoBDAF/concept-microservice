# Routing Keys

## Exchanges and Queues

| Routing Key | Exchange | Queue | TTL | Prefetch | Worker |
|-------------|----------|-------|-----|----------|--------|
| `document.process` | `document-tasks` | `document-processing` | 12h | 1 | graphrag-service |
| `email.send` | `email-tasks` | `email-processing` | 1h | 5 | email-worker |
| `image.process` | `image-tasks` | `image-processing` | 6h | 1 | image-worker |
| `profile.task` | `profile-tasks` | `profile-processing` | 1h | 2 | profile-worker |

## Notes
- Exchange types are **direct**.
- Queues are durable and include DLQ configuration.
- Prefetch is tuned to worker processing time.
