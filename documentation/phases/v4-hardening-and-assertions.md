# Phase v4 — Hardening & scored experiments

**Status:** pending · **Depends on:** v3 · **Exit tag:** `lab-v4.0` ·
**Decisions in force:** ADR-008 (all), ADR-009.1/.2/.6/.7, ADR-004 (all),
ADR-010.2 (CI phase 2)

## Mission

Two intertwined tracks. **(A) Architecture hardening:** fix the design flaws
the conceptual review + experiments proved (messaging retry/idempotency/
outbox/topology, JWT model, auth schema) — each fix lands *with the
experiment that demonstrates it*. **(B) Experiments v2:** the catalog becomes
machine-scorable (`make experiment E=<id>` → pass/fail) with chaos injection
and honest load. This is the largest pending phase; the two tracks interleave
naturally (a fix without its experiment doesn't count).

## Suggested internal order

Topology first (everything else declares against it), then retry/idempotency,
then outbox+results, then auth, then the assertion runner + chaos + loadgen,
then CI phase 2.

## Work breakdown — track A: hardening

1. **definitions.json topology (ADR-008.4):** author the complete topology
   (4 exchanges, work queues, retry queues per 008.1, DLQs, the new
   `task-results` + `expired` queues) as RabbitMQ definitions; mount in
   compose + ConfigMap on kind; strip declaration code from api-service,
   workers, graphrag (verify-only); regenerate ROUTING_KEYS.md from it.
2. **TTL policy (ADR-008.5):** in the definitions — no TTL on work queues;
   email keeps staleness TTL dead-lettering via distinct routing key to
   `email-expired` (not the poison DLQ). Update EXP-10 accordingly.
3. **Retry queues (ADR-008.1):** `<queue>.retry.{5s,30s,2m}` wait-queues
   dead-lettering back; workers/graphrag on failure publish to the right
   retry tier based on x-death count; after N → DLQ. Remove the x-max-retries
   pseudo-arg everywhere.
4. **Idempotency (ADR-008.2):** shared consumer guard — Redis SETNX on
   envelope id (TTL ~24h) in Go common + graphrag; profile processor becomes
   a real upsert as the natural-idempotency example.
5. **Outbox + results (ADR-008.3):** api_db `outbox` table written in the
   same tx as domain writes; relay goroutine publishes with confirms + marks
   sent; workers/graphrag publish completion/failure to `task-results`;
   api-service consumes it and advances document status
   (pending→processing→completed/failed) — closing the EXP-11 write-only
   finding. Generic endpoint whitelist + delete default-tasks (ADR-008.6).
6. **JWKS (ADR-009.1):** auth-service: RS256 keypair (from init-secrets),
   kid header, /.well-known/jwks.json; api-service: local verification with
   cached JWKS (refresh on unknown kid), introspection client kept behind a
   flag for strict mode. Update CONTRACTS §3.
7. **Sessions/revocation (ADR-009.2):** refresh-token rotation against a
   sessions table, reuse detection, logout deletes session. **Salt column
   dropped** (ADR-009.6) in the same migration wave. **Roles (ADR-009.7):**
   role middleware; admin-only user-management + destructive routes; seeded
   admin.

## Work breakdown — track B: experiments v2

8. **Definition format (ADR-004.2):** `experiments/<id>.yaml` (id, needs,
   steps as commands, watch refs, assertions: PromQL/HTTP/CLI checks with
   thresholds + timeouts); migrate EXP-01..12; EXPERIMENTS.md becomes
   generated index + prose (keep prose files).
9. **Runner (ADR-004.1):** `make experiment E=exp-04` executes steps, polls
   assertions, exits pass/fail with a junit-ish report; `make experiments`
   lists; results append to documentation/experiments/ log.
10. **Chaos Mesh (ADR-004.3):** install on kind; faults as experiment steps
    (PodChaos kill, NetworkChaos delay/partition); at least the api⇄postgres
    latency drill authored.
11. **Go loadgen (ADR-004.4):** `operational-workers/cmd/loadgen` — AMQP
    publisher (rate, duration, routing key, confirm mode, envelope-correct);
    containerized; wired as a step type for queue-side experiments.
12. **CI phase 2 (ADR-010.2):** Actions job booting compose and running the
    scored smoke subset (EXP-01, 02, 05 at least) on push to main.

## Out of scope

AWS anything (v5), UI beyond what v3 shipped (v6), guests beyond hello-guest
regression (v7), template extraction (v8), mesh/mTLS (rejected ADR-009.4).

## Exit experiments

- **EXP-40 — Retry & backoff:** worker's dependency made flaky (chaos or
  fault flag); message retries through 5s/30s/2m tiers, succeeds on recovery;
  DLQ only after N genuine failures. Assert via x-death counts + timing.
- **EXP-41 — Duplicate delivery:** kill a worker between processing and ack
  (chaos); redelivery is deduped by the SETNX guard; then flush Redis
  mid-flood and show the upsert processor stays correct anyway.
- **EXP-42 — Outbox crash-consistency:** kill api-service between DB commit
  and (old-style) publish window under upload load; relay recovers and
  publishes exactly the committed events; **document status reaches
  completed** end-to-end (EXP-11's gap closed).
- **EXP-43 — The incident, retired:** EXP-08 rerun under JWKS — throttle
  auth-service however you like; API requests keep flowing (local verify);
  introspection strict-mode flag restores the old behavior for comparison.
- **EXP-44 — Network chaos:** 200ms api⇄postgres latency injected; p99 and
  pool metrics respond per SLO doc; alert fires; remove fault, recover.
- **EXP-45 — Scored mode:** `make experiment E=exp-02` (and the CI subset)
  pass mechanically; deliberately break an assertion and watch it fail (prove
  the runner isn't a rubber stamp).

## Acceptance

- [ ] EXP-40..45 pass scored; EXP-01..12 migrated to YAML and still pass
- [ ] Topology exists only in definitions.json (grep proves no declares left)
- [ ] Document lifecycle: upload → completed status visible via API
- [ ] JWKS live; introspection opt-in; salt column gone; admin routes enforced
- [ ] CI runs verify + drift check + scored smoke subset
- [ ] Tag `lab-v4.0`
