# Phase v4 — deferred validation & follow-ups

**Context:** v4 was implemented in a parallel-agent pass (2026-07-19) on top
of the `phase/v4` skeletons: all track-A hardening code, the experiment
YAML catalog, the scored runner, chaos/loadgen/CI wiring landed and were
verified statically (builds, unit tests, `make verify`, `make drift-check`,
kustomize renders, an isolated-broker load test of `definitions.json`).
Live-stack execution was deliberately deferred: the machine's kind cluster
and compose ports were in use by the parallel v3 session, and v3's final
state (PR #2) had not merged when this pass ran. This file is the honest
ledger of what remains before `lab-v4.0` can be tagged.

**How to run all of this:** the phase doc now carries a step-by-step
[execution guide](v4-hardening-and-assertions.md#execution-guide--live-run-validation-round)
(operator + support session protocol, block sequencing, defect handoffs,
budget/abort criteria). This file remains the checklist; the guide is the
runbook.

## Must run before tagging lab-v4.0

| Item | What to do | Doc/entry point |
|---|---|---|
| Broker topology cutover | First `make up` after this lands needs `make nuke` first (queue-arg changes hit PRECONDITION_FAILED on an existing broker); on kind recreate the rabbitmq STS | v4-HANDOFF.md ground rules |
| EXP-01..12 rerun, scored | `make experiment E=exp-NN` for the migrated catalog; fix assertion calibration where live values disagree | experiments/README.md |
| EXP-40 retry & backoff | Flaky worker via `FAIL_FIRST_N_ATTEMPTS` (or PodChaos after chaos-up); watch 5s/30s/2m tiers + x-death counts | EXPERIMENTS.md §EXP-40 |
| EXP-41 duplicate delivery | `kill -9` worker between process and ack mid-flood; SETNX dedupe metric ticks; flush Redis → upsert stays correct | EXPERIMENTS.md §EXP-41 |
| EXP-42 outbox crash-consistency | Kill api-service in the commit→publish window under upload load; relay recovers; document reaches `completed` | EXPERIMENTS.md §EXP-42 |
| EXP-43 the incident, retired | EXP-08 rerun under JWKS local verify; flip `API_AUTH_STRICT_INTROSPECTION=true` to reproduce the old cascade | EXPERIMENTS.md §EXP-43 |
| EXP-44 network chaos | `make chaos-up`, apply `deploy/chaos/networkchaos-api-postgres-200ms.yaml`, watch p99/pool metrics + alert, remove, recover | EXPERIMENTS.md §EXP-44 |
| EXP-45 scored mode | `make experiment E=exp-02` passes; break a threshold → runner fails (not a rubber stamp); restore | v4-HANDOFF.md §B2 |
| Chaos Mesh first install | `make chaos-up` needs network (helm charts) and the kind cluster free | scripts/cluster/chaos-up.sh |
| CI phase 2 proof | `scored-smoke` job only runs on push to `main` — observable after the PR stack merges | .github/workflows/ci.yml |
| Loadgen live check | 200 msg/s × 30s flood visible in Queue depth panel; confirm-mode throughput delta observable | v4-HANDOFF.md §B4 |

## Reconciliation with v3 — DONE (2026-07-19)

v3 completed (PR #2: exit runs EXP-30..34 passed, SLOs calibrated) and was
merged into `phase/v4` the same day (merge commit `30f7914`). Resolutions
carried on the v4 side: v3's trace_id-in-DLQ-log fix re-applied inside
v4's rewritten failure paths (`routeFailure` threads the span's trace_id
into the retry-scheduled and DLQ-routing log lines — the EXP-31 pivot
holds across retry/DLX routing); `Dockerfile.loadgen` builder bumped to
golang:1.25-alpine matching v3's toolchain bump; go.mod is the union
(go 1.25.0 + go-redis). Full battery re-ran green post-merge (verify,
-race, drift-check, renders, runner gates, acceptance greps).

## Deviations from the handoff accepted at implementation

- **Idempotency key is attempt-scoped** — `idem:<queue>:<id>:<attempt>`
  (attempt = x-death retry count), not ADR-008.2's literal envelope-id key.
  With guard-before-process, a plain key would treat every 5s/30s/2m retry
  pass as a duplicate and ack it unprocessed (message loss; EXP-40
  impossible). Same-attempt redeliveries (crash between process and ack)
  and operator replays still dedupe. Go and Python implementations mirror
  byte-for-byte (`KeyForAttempt` / `key_for_attempt`).
- **Known dedupe blind spot:** a crash *after* processing but *before* the
  task.result publish leaves the guard claimed; the redelivery is acked as
  duplicate without re-emitting the result, so a document task can stick in
  `processing` until the guard TTL (24h) lapses or Redis is flushed.
  Accepted: the window is milliseconds-wide, the alternative (result
  duplication is already tolerated) would require claim-after-publish and
  reopen the double-side-effect window EXP-41 exists to catch.
  Deliberate asymmetry: operational workers DO emit `completed` for a
  deduped delivery (their results don't drive status; improves liveness if
  the original result was lost), graphrag deliberately does NOT — a
  mid-process crash also presents as a dup, and marking a possibly
  half-processed document `completed` would corrupt status silently;
  stuck-in-`processing` is visible and drillable instead.
- **Outbox relay restores tracing headers, not all AMQP properties** — the
  relay re-attaches `traceparent`/`tracestate`/`baggage` + MessageId from
  the stored envelope (v3 end-to-end traces hold across the broker hop);
  `CorrelationId`/`Priority` AMQP properties are not restored (both live in
  the envelope body; nothing reads them from AMQP properties).
- **`task-results` consumer acks poison** (no DLQ is bound to it by
  design); unknown documents and terminal-state repeats are dropped
  idempotently with a log line.
- **Some migrated assertions are contract-faithful but environment-
  sensitive** (clean-DLQ baselines, definitions-loaded topology); each
  such YAML carries an inline comment. Calibrate on the first live run.

## Known caveats shipped knowingly

- **definitions.json declares user `guest`** (guest/guest hash) because
  `load_definitions` skips default-user creation. Compose matches. On kind,
  init-secrets rotates the rabbitmq password — regenerate the definitions
  ConfigMap with `scripts/rabbitmq/generate-definitions.py --password <p>`
  or accept that the definitions-declared hash is overridden (see
  init-secrets.sh comments; wired note in v4-HANDOFF §A1).
- **Salt column drop invalidates pre-v4 dev credentials** (ADR-009.6,
  accepted for the lab; noted in the migration header). Re-register or use
  the seeded admin (`SEED_ADMIN_EMAIL`/`SEED_ADMIN_PASSWORD`).
- **`loam validate` hook noise** on `documentation/decisions/*` writes:
  pre-existing on main, not v4's.

## Nice-to-haves consciously skipped

- Outbox relay LISTEN/NOTIFY (poll loop is enough at lab scale; noted in
  outbox.go).
- Redis-backed auth lockout (parked in PRD §8 unless auth scales >1
  replica in drills).
