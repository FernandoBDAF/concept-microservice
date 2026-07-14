# Conceptual Review

**Date:** 2026-07-10 · **Scope:** architecture and design concepts, not code
bugs (those were fixed in the [2026-07 refactor](../refactor/2026-07-full-refactor.md)).

This project was built as a Kubernetes-learning and deployment-planning lab
with generic, reusable infrastructure pieces (auth, queue, cache, storage).
Judged against that goal, here are the conceptual issues — roughly ordered by
how much they matter. Several are *tensions* rather than mistakes; each item
says what to do about it, and the open decisions are collected in the
[PRD](../PRD.md).

## 1. The consolidation archived the lab it was meant to serve ⭐ biggest issue

The move from six microservices to one consolidated api-service was justified
on performance grounds ("~10x faster, no HTTP overhead") — a fine argument for
a *product*. But the project's stated purpose was practicing Kubernetes
operations: service-to-service traffic, network policies, per-service scaling,
rollouts. The consolidation didn't just simplify the architecture; it deleted
the practice substrate — kind configs, ingress, metrics-server, zero-trust
network policies, per-service manifests, k6 jobs and their analyses all moved
to `legacy_project/` and were never adapted to the new shape.

Notably, the old lab was *good*: manifests had resource requests/limits,
liveness/readiness probes, security contexts, default-deny network policies,
and careful educational annotations. The mistake wasn't quality — it was
optimizing for a goal (runtime performance) that the project doesn't actually
have, at the cost of the goal it does have.

**Direction:** keep the consolidated architecture (it's defensible and built
now), but restore the cluster lab around it (PRD v2). The current five
deployables (api, auth, graphrag, 3 workers) still give you scaling, policies,
probes, and rollout practice.

## 2. Token validation reintroduces the hop the consolidation removed

Every authenticated request to api-service makes a synchronous HTTP call to
auth-service (`POST /v1/auth/token/validate`), with a circuit breaker to guard
it. Two conceptual problems:

- It contradicts the consolidation's own rationale: the one HTTP dependency
  that remained is on the hot path of *every* request. Auth-service becomes a
  runtime single point of failure for the whole API (breaker open = everyone
  locked out).
- JWTs are specifically designed to avoid this: they're self-contained,
  signed claims. The standard pattern is asymmetric signing (RS256/ES256) —
  auth-service holds the private key, api-service verifies locally against a
  published JWKS, and the hop disappears. Remote introspection is the tool for
  *revocation-sensitive* checks, not for every request.

Today the JWT is HS256 with a single `JWT_SECRET` that only auth-service
knows, which *forces* the remote call (sharing an HS256 secret across services
would let any service mint tokens — worse). **Direction:** RS256 + JWKS
endpoint, local verification in api-service, keep introspection as an optional
strict mode. Open questions in PRD §OQ-auth.

*Empirically confirmed during v1 verification:* `/token/validate` carried a
**hardcoded** rate limit of 100/minute (`tokenValidationRateLimit`). A 1-VU k6
smoke run exhausted it (metrics showed exactly 100×200 then 429s), and a 50-VU
burst failed 96.7% of API requests: api-service's circuit breaker converted
the 429s — a capacity signal — into an open breaker, 401-ing the entire API.
Three design smells in one incident: per-request introspection puts login-path
throttling on the hot path of every API call; a service-to-service budget was
hardcoded rather than configured; and the breaker counts throttling as
failure, amplifying backpressure into outage. v1 makes the limit configurable
(`TOKEN_VALIDATION_RATE_LIMIT_MAX`, lab-sized in compose); the real fixes are
OQ-A1 (local JWT verification) and OQ-A4 (rate-limit architecture).

## 3. `x-max-retries` is fiction — there is no retry mechanism

Queues are declared with an `x-max-retries` argument. RabbitMQ has no such
feature: it stores the header but enforces nothing. Combined with the
consumers' (correct) poison-message policy — `nack(requeue=false)` → DLQ — the
actual behavior is: **a message that fails once goes straight to the DLQ. The
"3 retries" exist only in documentation.**

Worse, the phantom argument still participates in RabbitMQ's
byte-identical-declare-args rule, so it makes every topology declare more
fragile for zero benefit.

**Direction:** decide a real retry model (PRD §OQ-messaging): x-death-header
counting with redelivery, a retry queue with per-queue TTL backoff, or the
delayed-message-exchange plugin — then delete `x-max-retries` or make it real.

## 4. TTL on work queues silently discards work

Task queues carry `x-message-ttl` (email 1h, profile 1h, image 6h, document
12h). On a work queue this means: if a worker is down (or backlogged) longer
than the TTL, pending jobs are dead-lettered — expired *work* lands in the
same DLQ as *poison* messages, indistinguishable. An email that waited 61
minutes isn't a failure; it's just late.

**Direction:** for a lab this is actually a nice scenario generator, but the
semantics should be deliberate: either drop TTLs on main queues, or route
expirations differently from failures (different dead-letter routing key), and
define per-task staleness policy (an email may expire; a document-processing
job probably shouldn't).

## 5. Both sides declare topology — the byte-identical trap

Publisher (api-service) and every consumer independently declare exchanges,
queues, and DLQs, and RabbitMQ requires their arguments to match exactly. This
is a documented-and-hit failure mode: the `profile.task` drift (publisher on
`tasks-exchange`, docs/consumer on `profile-tasks`) made an entire pipeline
silently dead, and mismatched TTL args would have produced
`PRECONDITION_FAILED` crashes.

**Direction:** single ownership of topology. Options (PRD §OQ-messaging):
RabbitMQ `definitions.json` loaded at broker boot (topology as infrastructure,
services declare nothing), or a shared Go/Py library generated from
`ROUTING_KEYS.md`, or the convention "consumer owns queue, publisher owns
exchange only." The repo now at least has a single canonical table
(`graph-worker/shared/contracts/ROUTING_KEYS.md`) and contract docs, but
nothing *enforces* code↔doc agreement — a contract test in CI should.

## 6. At-least-once delivery, but consumers aren't idempotent

Envelopes carry a UUID `id`, yet no consumer deduplicates on it. RabbitMQ
redelivers on connection loss after processing-but-before-ack, so every
worker will eventually process some message twice. Harmless for today's
simulated processors; a real defect in the "generic reusable worker" goal.

**Direction:** idempotency keyed on envelope `id` (Redis SETNX with TTL, or a
processed-ids table) as part of the worker template (PRD v5).

## 7. The auth `salt` column adds nothing

`users.salt` (VARCHAR(64)) is concatenated into `bcrypt.hash(password + salt)`.
bcrypt already generates and embeds a per-password salt; a second salt stored
*in the same row as the hash* adds no security against any attacker who has
the row. If the intent was a *pepper* (secret separate from the DB), it must
live outside the DB (env/KMS). As-is it complicates the schema and password
verification for zero gain.

**Direction:** drop the column (or promote it to a real env-based pepper) next
time the auth schema is touched. Also note: `role` is stored and put into the
JWT, but no endpoint enforces roles — authorization is a missing concept, and
in-memory rate limiting/lockout counters won't coordinate across replicas
(fine single-instance; wrong once k8s scales auth-service to 2+).

## 8. Observability was declared but never closed the loop

Every service defines Prometheus metrics, and the envelope carries a
`trace_id` — but until v1 there was no Prometheus to scrape, no Grafana to
look at (the "monitor with a UI" goal), the Go workers never even exposed
their `/metrics` route, and `trace_id` is generated per message rather than
propagated from the incoming HTTP request, so nothing can actually be traced
end-to-end.

**Direction:** v1 (this change) adds the Prometheus+Grafana loop and worker
`/metrics`. Real distributed tracing (OpenTelemetry, propagate context
HTTP→AMQP→worker) and log aggregation are PRD v3; until then `trace_id` is
decorative.

## 9. Storage-consistency is unaddressed (three stores, no outbox)

A document upload writes the blob to MinIO, metadata to Postgres, and then
publishes `document.process`. There's no transactional outbox or saga, so a
crash between steps leaves orphaned blobs or metadata-without-event; graphrag
then writes to a fourth store (Mongo) with no reconciliation path. For the
reusable-infra goal, the outbox pattern (publish from a DB-committed events
table) is the standard answer — and a great lab scenario to build.

Related smaller points: profile list cache is per-page cache-aside with
SCAN-based invalidation (O(keyspace), no stampede protection — fine at lab
scale, know why it doesn't scale); both logical DBs share one Postgres
instance (one failure domain — fine locally, name it in the k8s design).

## 10. Configuration duplication with no single source of truth

The same topology/env facts live in: compose file, per-service k8s manifests,
CONTRACTS.md, per-service config defaults, and (historically) six deployment
guides. They drift — that's how the profile-tasks incident happened, and why
the k8s manifests are already stale again (pre-v1 they lacked the 8081
metrics port). **Direction:** pick the layering (PRD §OQ-platform): contracts
doc → generated/validated config (kustomize overlays for k8s, compose for
local), plus a CI check that greps declared names against the canonical table.

## 11. Committed secrets, even educational ones, are habit-forming

The old lab committed base64 k8s Secrets (JWT keys, DB passwords) with
"educational" framing, and today's compose/`.env.example` carry default
dev credentials (fine) — but there's no stated line between "lab default" and
"never commit." Since the explicit goal is *reusing these pieces in real
projects*, the template should bake in the good habit: secrets generated at
setup (`make init-secrets`), SOPS/sealed-secrets for k8s, `.env` gitignored
(done in v1).

## 12. Smaller conceptual notes

- **Generic `SubmitTask` endpoint** accepts arbitrary task types that fall
  through to a `default-tasks` parking-lot exchange with no consumer — an
  unvalidated write path to the broker; either whitelist task types or
  document the parking lot as intentional.
- **Prefetch=1 + 12h processing budget** for graphrag means one stuck document
  blocks the queue for hours; consider per-message timeouts (PRD v4 scenario).
- **`api-service` proxies document downloads** through itself; presigned MinIO
  URLs would offload that (deliberate simplicity is fine — note it).
- **Worker HTTP servers use gin** (a full web framework) to serve two health
  routes — `net/http` would do; matters only for the reuse-template's
  dependency footprint.
- **Auth register lives at `POST /v1/users`** while docs/tests colloquially
  call it "register"; harmless, but the OpenAPI docs (swagger) should be the
  single canonical reference and be linked from the README.
- **README/plan docs historically described aspiration as fact** (e.g., "make
  docker-push", monitoring guides for stacks that didn't exist). The refactor
  moved these to `documentation/planning/`; keep the rule "top-level docs
  describe only what runs."
- **Document status is write-only** (found by EXP-11): documents are created
  `pending` and nothing can ever advance them — graphrag has no path back to
  `api_db`. The status/`processing_started_at`/`error_message` columns and
  the status endpoint imply a lifecycle that doesn't exist. Needs a results
  channel (worker→API callback, a status queue, or shared-DB decision) —
  pairs with the outbox question (OQ-M3).
- **Publisher-confirm timeouts surface as unlogged 500s** (v1.1 exit run,
  EXP-03/04): when a RabbitMQ confirm misses the 5 s `ConfirmTimeout`, the
  task endpoint returns 500 but emits no error-level log — only the info
  access line betrays it (latency ≈ 5.01 s). ~0.06–0.17% of publishes under
  10–50 VU load. Ties into §8 (observability loop) and the v4 retry/outbox
  work; at minimum the publish path should log the confirm failure.
- **Cache outage = silent latency amplification** (EXP-12 discovery,
  [write-up](../experiments/2026-07-10-cache-outage-latency-amplification.md)):
  cache-aside fallback works (0% errors) but requests pay 0.3–4 s go-redis
  dial penalties; client logs bypass zap; only `/ready` tells the truth.
  Fail-fast cache path + `redis.SetLogger` are v4-hardening candidates;
  latency-based SLIs (v3) are the operational tell.
- **`/ready` ANDs every dependency → k8s amplifies broker outage to full
  API outage** (EXP-20 on kind,
  [write-up](../experiments/2026-07-10-readiness-coupling-broker-outage.md)):
  compose kept CRUD serving through a RabbitMQ outage; on Kubernetes the
  same `/ready` fails the readinessProbe, the Service loses every endpoint,
  and nginx 503s the routes that never needed the broker. Readiness should
  gate on what all routes need (PG), or the app should serve degraded —
  decide in v4; the v8 template must not ship the AND.
