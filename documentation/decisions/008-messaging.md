# ADR-008 — Messaging architecture (2026-07-10)

## 008.1 Retry model: TTL-backoff retry queues
**Context:** `x-max-retries` was fiction — one failure went straight to DLQ
(review §3). **Decision:** on failure, publish to `<queue>.retry.{5s,30s,2m}`
wait-queues that dead-letter back to the work queue; after N attempts
(x-death count) → DLQ. Pure RabbitMQ, real exponential backoff.
**Consequences:** transient failures self-heal; topology grows (defined once
via 008.4); this becomes the v8 worker template's retry story.

## 008.2 Idempotency: Redis SETNX guard + naturally-idempotent processors
**Context:** at-least-once delivery guarantees duplicates; envelope ids exist
but nothing dedupes (review §6). **Decision:** default guard = SETNX on
envelope id with ~24h TTL in shared Redis; processors additionally written as
upserts where possible. **Consequences:** cheap dedup with honest window
semantics; "flush Redis mid-load" becomes a great drill; email-like
non-idempotent actions rely on the guard alone.

## 008.3 Transactional outbox + results queue (v4)
**Context:** MinIO write, Postgres write, publish are three unrelated steps
(review §9); EXP-11 proved document status is write-only (no results path).
**Decision:** api-service gets an events/outbox table written in the DB
transaction + a relay publisher (in-process goroutine; no Debezium at this
scale). Workers publish to a `task-results` queue; api-service consumes it to
advance document status. **Consequences:** crash-consistent publishing AND
the status lifecycle finally real — one design closes two findings.

## 008.4 Topology ownership: broker-loaded definitions.json
**Context:** dual declaration + byte-identical args caused the dead
profile-tasks pipeline (review §5). **Decision:** the full topology
(exchanges, queues, bindings, retry/DLQ args) lives in a versioned
definitions.json loaded by RabbitMQ at boot (compose mount + k8s ConfigMap);
services stop declaring and only verify/consume. ROUTING_KEYS.md becomes
generated documentation. **Consequences:** the drift class dies; topology
changes are reviewable infrastructure diffs.

## 008.5 TTL policy: no TTL on work queues by default
**Context:** EXP-10 showed queue TTLs silently discard valid work into the
poison DLQ (review §4). **Decision:** work queues get no message TTL —
backlog is an alerting problem, not a discard problem. Exception: email keeps
a staleness TTL but expiry dead-letters via a distinct routing key to an
`expired` queue, never mixed with poison. **Consequences:** slow days lose no
work; staleness is explicit and observable where it's real.

## 008.6 Generic task endpoint: whitelist; parking lot deleted
**Context:** POST /profiles/:id/tasks accepted arbitrary types into a
consumer-less `default-tasks` exchange (review §12). **Decision:** the
endpoint accepts only the four contract task types (400 otherwise);
default-tasks exchange and fallback path are removed. **Consequences:**
no unvalidated write path to the broker; new task types are deliberate
contract changes.
