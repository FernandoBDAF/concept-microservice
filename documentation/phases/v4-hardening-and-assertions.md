# Phase v4 — Hardening & scored experiments

**Status:** implemented (2026-07-19, PR #3: skeletons + full implementation
+ v3-final reconciled; static battery green) — live-run validation pending,
run it via the [execution guide](#execution-guide--live-run-validation-round)
below; deferred items ledgered in [v4-DEFERRED.md](v4-DEFERRED.md) ·
**Depends on:** v3 (merged) · **Exit tag:** `lab-v4.0` ·
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

---

## Execution guide — live-run validation round

*(Amended 2026-07-19, after the implementation pass. This is the runbook
for everything `v4-DEFERRED.md` registers; run it with the same
orchestrated methodology that built the phase. Budget: 4–6h wall-clock.
The runs are sequential by nature — one shared, stateful stack — so
subagents parallelize authoring, write-ups, and fixes, not the runs.)*

### Session topology

Two Claude sessions, strict resource ownership:

- **Operator session** (primary) — the ONLY session that touches live
  state (`docker`, `kind`, `make up/nuke/cluster-*`). Coordinates: an
  *author* agent (writes `experiments/exp-40..45.yaml` + the EXP-40..45
  prose sections of EXPERIMENTS.md before runs start — disjoint files),
  and drives the runner itself (or via one *operator* agent that owns all
  stack commands). The orchestrator sequences blocks, classifies failures
  (calibration vs defect), reviews every fix diff, keeps the run log.
- **Support session** (on-call, separate worktree) — code-only, NEVER
  runs compose/kind. Receives defect handoffs, fixes + unit-tests, runs
  the static battery (`make verify && make drift-check`), commits to
  `phase/v4`, answers "rebuild needed: <images>". Keeps fix cycles from
  stalling the stack and keeps the operator session's context clean.
- Coordination rules: any other session (v5, docs) must not touch the
  stack or push to `phase/v4` while this runs. Both sessions
  `git pull --rebase` before commits. **If a fix touches
  `deploy/rabbitmq/definitions.json` → operator must `make nuke` (compose)
  / recreate the rabbitmq STS (kind) before rerunning** (PRECONDITION_FAILED
  ground rule, v4-HANDOFF).

### Phase 0 — preconditions (~15–25 min)

1. Stack ownership confirmed free: `docker ps` shows no other session's
   containers you didn't expect; kind cluster `lab` idle.
2. Fresh `phase/v4`; baseline `make verify && make drift-check` green.
3. **`make nuke`** (mandatory first time — old broker volume carries
   pre-v4 queue args) → `make up` (first boot: `gen-jwt-keys.sh` seeds
   `.env`, definitions.json loads, images rebuild on go 1.25).
4. Gates: `make queues` → **22 queues**; `docker compose ps` all healthy;
   `curl -s localhost:3000/.well-known/jwks.json` → non-empty `keys`
   (RS256 live); `curl -s localhost:8080/ready` → 200.
5. `make experiment E=exp-01` as the smoke gate. A definitions/topology
   failure here is a defect, not calibration — handoff immediately.

### Block 1 — scored catalog rerun, EXP-01..12 (~1.5–2.5h)

Run `make experiment E=exp-NN` in order 01→12 (runner appends
`documentation/experiments/RUNS.md` + junit to `.experiment-results/`).
On any assertion failure, classify FIRST:

- **Calibration** (threshold vs live reality — expected for exp-05/06/10/11,
  see their YAML comments): operator adjusts the YAML threshold, records
  old→new in the run log, reruns. No support handoff.
- **Defect** (behavior wrong): handoff to support (format below), continue
  with the next experiment if stack state allows, rerun after the fix.

Special proof points: **exp-10** must show the A2 split (expiry →
`email-expired`, reason `expired`; poison → `email-processing.dlq`);
**exp-11** must end with the document at `completed` via the API — that is
A5's live proof (the old EXP-11 write-only gap).

### Block 2 — exit experiments, compose-side: EXP-40..43 + 45 (~2–3h)

Author agent first: `exp-40..45.yaml` + prose (schema:
`experiments/README.md`; encode every mechanical Expect as an assertion;
x-death header inspection stays `watch`). Then run:

- **EXP-40 retry & backoff (~30–45m):** two flavors on email-worker.
  Recovery: `FAIL_FIRST_N_ATTEMPTS=2` (env on the service, restart it),
  publish one email task → fails, waits 5s tier, fails, waits 30s tier,
  succeeds on 3rd attempt; assert `email_retries_total{tier}` ticks and
  final delivery; DLQ untouched. Exhaustion: `FAIL_FIRST_N_ATTEMPTS=99`
  → all three tiers then DLQ (reason: explicit publish), assert
  `email_dlq_total` + x-death sums (watch). Unset the env after.
- **EXP-41 duplicate delivery (~30–45m):** flood profile tasks (loadgen);
  `docker kill` (SIGKILL) profile-worker mid-flood; on restart assert
  `profile_duplicates_total` > 0 and no double side effects (upsert
  count == task count); then `docker compose exec redis redis-cli FLUSHALL`
  mid-flood → processing stays correct (natural idempotency), only the
  duplicates metric loses memory. 
- **EXP-42 outbox crash-consistency (~30–45m):** loop
  `scripts/simulate/document-upload.sh` for load; `docker kill`
  api-service repeatedly to land in the commit→publish window; restart;
  assert `api_outbox_pending` drains to 0, every committed document
  reaches `completed`, and counts match (documents == task.results
  consumed — duplicates tolerated, losses not).
- **EXP-43 the incident, retired (~30–40m):** EXP-08 levers
  (`TOKEN_VALIDATION_RATE_LIMIT_MAX` low) under sim-load. Default (JWKS
  local verify): API keeps serving, p99 within SLO, breaker quiet. Flip
  `API_AUTH_STRICT_INTROSPECTION=true` (recreate api-service): the old
  cascade reproduces. Flip back. Assert both directions.
- **EXP-45 scored mode (~10–15m):** `make experiment E=exp-02` passes;
  edit one threshold to an impossible value → runner exits non-zero
  (prove it's not a rubber stamp); `git checkout` the YAML; confirm
  RUNS.md + junit artifacts exist for the session's runs.

### Block 3 — kind-side + chaos: EXP-44 (~1–1.5h)

Compose and kind can coexist, but under WSL memory pressure run this
after Block 2 (obs stack + OpenSearch are heavy; see `.wslconfig` note in
README dependencies).

1. `make images` (v4 images → :5001). Broker: regenerate definitions with
   the rotated password — `python3 scripts/rabbitmq/generate-definitions.py
   --password "$RABBITMQ_PASSWORD"` (from `.lab-secrets.env`) — rebuild the
   rabbitmq ConfigMap, recreate the STS (delete pod + PVC if args changed);
   or accept the committed guest hash per the ledger note. Rerun migration
   jobs (000003 + 002 land).
2. Gates: `make cluster-status` clean; `make cluster-queues` → 22;
   `make cluster-sim-smoke` passes.
3. `make chaos-up` (first install, needs network; pinned 2.8.3) →
   `kubectl apply -f deploy/chaos/networkchaos-api-postgres-200ms.yaml` →
   watch p99 + pool metrics respond (SLO p99 3s, calibrated in v3), the
   PrometheusRule alert fires → ntfy; `kubectl delete -f …` → recovery,
   alert clears. Optional: `podchaos-kill-worker.yaml` as EXP-40's chaos
   flavor.

### Block 4 — close-out (~30–45 min)

1. Write-up: `documentation/experiments/2026-MM-DD-v4-exit-runs.md`
   mirroring the v3 exit-runs file (author agent drafts from RUNS.md +
   run log; orchestrator edits; include calibration changes and fix
   commits per experiment).
2. Docs: regen index if YAMLs changed (`make routing-keys` untouched);
   flip `v4-DEFERRED.md` must-run table to done-with-dates; phase doc
   Status → validated; EXPERIMENTS.md prose corrections found live.
3. Final `make verify && make drift-check`, push `phase/v4` (PR #3
   updates), owner merges → CI `scored-smoke` proves itself on the main
   push → tag the merge commit `lab-v4.0` and push the tag.

### Defect-handoff protocol (operator ↔ support)

Per defect, the operator appends to the run log (and pings support):
experiment id · failing assertion + actual output · suspected module ·
exact repro command · stack state (fresh/nuked, env flags). Support
replies with: fix commit on `phase/v4` · unit test added · static battery
result · `rebuild: <image list or none>`. Operator rebuilds only those
images (`docker compose build <svc> && docker compose up -d <svc>`, or
`make images` for kind), reruns the experiment, records the fix commit in
the write-up. Definitions changes trigger the nuke rule above.

### Budget and abort criteria

Historical defect yield of exit runs (v1.1: 2, v2: 3+2, v3: 4 fixes) says
budget 3–6 fix cycles inside the 4–6h. If one defect exceeds ~1h of
support time, or the machine is contended: stop cleanly, record what
passed, re-open `v4-DEFERRED.md` with the remainder — the ledger
convention explicitly allows an honest partial close. Never tag with a
red or unrun exit experiment.
