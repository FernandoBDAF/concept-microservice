# Experiments

Guided operational drills for the lab. Each experiment tells you **what to
run, what to watch, and what you should see** — running the catalog top to
bottom validates the implementation itself (PRD §5). Findings worth keeping
get a write-up in [`documentation/experiments/`](documentation/experiments/).

> This file is the v1.1 form of the experiment library. Mission Control
> (PRD v5) will later render these as runnable sessions in the UI.

## Before you start

```bash
cp .env.example .env   # once; needed by EXP-08
make up                # full stack (first run builds images, ~2-4 min)
make monitoring        # prints the three UI URLs
```

Open **Grafana → Lab Overview** (http://localhost:3001, admin/admin) and keep
it visible during every experiment; refresh is 10s. Also useful:
**RabbitMQ** http://localhost:15672 (guest/guest) and
**Prometheus → Status → Targets** http://localhost:9090/targets.

Conventions: run from the repo root · **Watch** names Lab Overview panels ·
reset anytime with `make nuke && make up` (destroys data) · after each
experiment run the Cleanup line, or carry state into the next one knowingly.

Time for the full catalog: roughly 60–90 minutes.

---

## EXP-01 · Cold start & baseline

**Goal:** a healthy, fully-observable stack from nothing — the baseline every
other experiment compares against.
**Validates:** compose orchestration, DB migrations, MinIO bucket init,
RabbitMQ topology declaration, every metrics endpoint and scrape target.

**Steps**
1. `make nuke && make up`
2. `docker compose ps` — expect 13 containers, infra `(healthy)`, no restarts.
3. Prometheus → Targets: **8/8 up** (api, auth, graphrag, 3 workers,
   rabbitmq, prometheus).
4. `make queues`

**Expect**
- 8 queues: 4 work queues (`*-processing`, consumers=1 each) + 4 DLQs
  (consumers=0), all with 0 messages.
- Lab Overview panels render (flat/empty is correct — nothing has happened).
- `curl -s localhost:8080/ready` → `postgres/rabbitmq/redis: ok`.

**If not:** a target down usually means that service crashed at boot —
`make logs S=<service>`.

---

## EXP-02 · Golden-path smoke

**Goal:** one user's journey end-to-end: register → login → authenticated
CRUD → async tasks consumed.
**Validates:** auth contract (§CONTRACTS 3), profile CRUD, publish path,
worker consumption — the whole happy path.

**Steps**
1. `make sim-smoke` (1 VU, 15 s)
2. Watch: **API request rate** (a small 2xx wave), **Worker throughput**
   (email + profile blips ~10–15 s later), **Queue depth** (stays ≈0 —
   consumption keeps pace at this rate).

**Expect:** k6 summary — all checks ✓, `http_req_failed: 0.00%`;
`make queues` back to 0 within ~30 s; DLQs untouched.

---

## EXP-03 · Steady load

**Goal:** the system at cruise: does capacity hold, do latencies stay flat,
does consumption match production?
**Validates:** connection pooling, cache behavior under repeat traffic,
worker prefetch tuning at sustainable rates.

**Steps**
1. `make sim-load` (10 VUs / 2 min; vary with `SIM_VUS=20 SIM_DURATION=5m`)
2. Watch: **API latency** (p95 should sit low and *flat* — drift up over the
   run would mean saturation or a leak), **Queue depth** (sawtooth near 0),
   **Worker throughput** (steady, matching input rate ÷ 2 since half the
   iterations submit tasks).
3. Optional: `docker stats --no-stream` mid-run for a memory snapshot.

**Expect:** checks ≥ 99%, `http_req_failed < 1%`, p95 well under the 800 ms
threshold (calibration: 50 VUs did p95 ≈ 44 ms on a dev laptop), queues drain
to 0 shortly after the run.

---

## EXP-04 · Burst absorption & drain

**Goal:** overload the intake faster than workers consume; watch the queues
do their job (buffer, then drain). This is *the* reason queues exist.
**Validates:** backpressure via queueing, durable delivery under load,
prefetch differences between workers.

**Steps**
1. `make queues` (confirm ≈0 baseline)
2. `make sim-burst` (50 VUs / 30 s — expect ~1,400 iterations, ~700 email +
   ~700 profile tasks)
3. Watch: **Queue depth** — `email-processing` and `profile-processing`
   climb into the hundreds, then drain after the burst ends.
4. Time the drain: `make queues` every ~30 s until 0. Note email (prefetch 5)
   vs profile (prefetch 2) drain slopes.

**Expect:** k6 `http_req_failed < 5%` (calibration: 0.03% at 5,567 requests,
~179 req/s); backlog peaks of several hundred; full drain in a few minutes;
**Dead-letter queues panel stays flat** — load is not failure.

**Write-up prompt:** drain rate per worker ≈ ? msg/s. That number is your
capacity baseline for EXP-07.

---

## EXP-05 · Poison messages & DLQ triage

**Goal:** feed every pipeline garbage; confirm poison lands in DLQs instead
of crashing consumers or redelivering forever. Then triage like an operator.
**Validates:** DLQ topology (`<exchange>.dlx` / `<queue>.dlq`),
nack-without-requeue policy, consumer crash-resilience, worker error metrics.

**Steps**
1. `make sim-poison` (3× two flavors — not-JSON and empty-payload — to all
   4 exchanges; prints queue table after)
2. Watch: **Dead-letter queues** panel steps up; **Worker errors** blips.
3. Triage in RabbitMQ UI → Queues → `email-processing.dlq` → *Get messages*:
   read the raw poison payload — this is how you'd diagnose a real producer
   bug.
4. `make logs S=email-worker` — find the parse/validation error lines.

**Expect:** `email/image/profile-processing.dlq` **+6 each**;
`document-processing.dlq` **+3** — graphrag ACK-drops structurally-valid
envelopes with invalid payloads *by design* (CONCEPTUAL_REVIEW §12 notes this
asymmetry; deciding whether to change it is OQ-M5's cousin). Work queues
return to 0; consumers stay alive (consumers=1 throughout).

**Cleanup:** purge DLQs in the RabbitMQ UI (or keep the count in mind for
later experiments).

---

## EXP-06 · Worker outage & recovery

**Goal:** a consumer dies with traffic flowing; nothing is lost; recovery
drains the backlog. The fundamental durability promise.
**Validates:** durable queues, reconnect-with-backoff, no-loss delivery,
consumer-presence visibility.

**Steps**
1. `make sim-outage` (stops email-worker → floods 100 messages → restarts →
   polls until drained; variants: `WORKER=image N=200`)
2. Watch: **Queue depth** for `email-processing` — vertical climb to 100
   while consumers=0, then a clean drain slope after restart; **Worker
   throughput** silent during the outage, spike after.
3. During the stopped phase, `make queues`: `email-processing` shows
   **consumers 0** — that's your "who's actually listening" signal.

**Expect:** script prints the backlog, then `drained ✔` within ~2 min; DLQs
flat (an outage is not an error); zero message loss (100 in → 100 processed).

---

## EXP-07 · Consumer scale-out

**Goal:** horizontal scaling of consumers — the competing-consumers pattern
live.
**Validates:** stateless workers, queue-distributed delivery, capacity
scaling ~linearly with replicas.

**Steps**
1. Baseline: `docker compose stop email-worker && python3
   scripts/simulate/publish.py flood --routing-key email.send --count 300 &&
   docker compose start email-worker` — time the drain (`make queues`).
2. Scale: `make scale S=email-worker N=3`, purge/let drain, repeat the same
   flood. Time it again.
3. Watch: **Queue depth** drain slopes; RabbitMQ UI shows
   `email-processing` consumers=3.

**Expect:** ~3× steeper drain with 3 replicas (prefetch 5 each).
**Observe & note:** the **Worker throughput** panel undercounts now —
Prometheus scrapes `email-worker:8080` as *one* static target, so replicas
behind the same DNS name are sampled arbitrarily. This is the concrete
argument for real service discovery — arriving with Kubernetes in PRD v2.

**Cleanup:** `make scale S=email-worker N=1`

---

## EXP-08 · Incident reproduction: rate limit → breaker → outage

**Goal:** re-create the real incident this lab caught during v1 verification
(CONCEPTUAL_REVIEW §2): throttling on the token-validate path cascades into a
full API outage via the circuit breaker. Practice diagnosing it from metrics.
**Validates:** breaker behavior, the monitoring loop's diagnostic power, and
your incident-response reading speed.

**Steps**
1. In `.env` set `TOKEN_VALIDATION_RATE_LIMIT_MAX=100`, then
   `docker compose up -d auth-service` (recreates with the tight limit).
2. `make sim-burst` — it will start failing part-way through.
3. **Diagnose before reading on.** Where you should land: **API request
   rate** shows a 401 wall; **Auth request rate** flat-lines while the API
   is "busy" (breaker open = validate calls stop leaving api-service);
   Prometheus query
   `sum by (status_code) (auth_service_http_requests_total{route="/token/validate"})`
   shows exactly 100×200 then 429s.
4. Restore: set it back to `100000`, `docker compose up -d auth-service`,
   `make sim-smoke` to confirm recovery.

**Expect:** during the incident, k6 fails 90%+ with 401s at ~2–4 ms (that
suspicious *speed* is the breaker's signature — rejections without upstream
calls); after restore, smoke is 100% green.

**Write-up prompt:** what would page first in production — 401 rate, breaker
state, or 429s? What's missing from the dashboard to answer in one glance?

---

## EXP-09 · Broker outage (partial degradation)

**Goal:** kill the message broker; see which half of the system dies and
which half doesn't. Degradation boundaries are the heart of architecture.
**Validates:** sync/async decoupling, readiness signaling, worker reconnect
loops, queue durability across broker restarts.

**Steps**
1. Leave a backlog first: `docker compose stop email-worker && python3
   scripts/simulate/publish.py flood --routing-key email.send --count 50`
2. `docker compose stop rabbitmq`
3. Probe the boundary (get a token via the login flow in
   `scripts/simulate/document-upload.sh`, or just watch panels during
   `make sim-smoke`): profile **CRUD still returns 200** (Postgres/Redis
   path), task submissions **fail** (publish path), `curl
   localhost:8080/ready` → rabbitmq not ok.
4. `make logs S=profile-worker` — reconnect attempts with growing backoff.
5. Prometheus Targets: rabbitmq target down (your "broker is gone" alert
   signal, OQ-O4).
6. `docker compose start rabbitmq && docker compose start email-worker`

**Expect:** clean partial degradation (reads/writes fine, async intake
failing with 5xx, readiness degraded); after restart, workers reconnect
without manual help and the 50-message backlog **survived the broker
restart** (durable queues) and drains.

---

## EXP-10 · Message-TTL expiry (work loss semantics)

**Goal:** watch messages *expire* out of a work queue with nobody failing —
the silent-work-loss semantics flagged in CONCEPTUAL_REVIEW §4, made visible.
**Validates:** dead-letter-on-expiry behavior; the DLQ ambiguity finding
(expired vs poison are indistinguishable there).

**Steps**
1. `docker compose stop email-worker`
2. `python3 scripts/simulate/publish.py flood --routing-key email.send
   --count 20 --expiration-ms 10000` (per-message TTL: 10 s — the queue's own
   1 h TTL is too slow to demo)
3. Watch **Queue depth** vs **Dead-letter queues** across ~15 s:
   `email-processing` 20 → 0 while `email-processing.dlq` +20, with **zero
   worker activity**.
4. In the RabbitMQ UI, Get Message on the DLQ: `x-death` header shows
   `reason: expired` — the only way to tell it apart from poison
   (`reason: rejected`).
5. `docker compose start email-worker`

**Expect:** all 20 dead-letter without processing. **Discussion:** these
were valid emails; production behavior would be "users silently never got
mail." OQ-M5 (per-type TTL semantics, separate expiry routing) is the fix
to design.

---

## EXP-11 · Document pipeline E2E

**Goal:** the full storage pipeline, never live-tested before v1.1: upload →
MinIO object → Postgres metadata → `document.process` event → graphrag
consumption.
**Validates:** multipart upload handling, MinIO integration, the
document-tasks contract, graphrag's consume path (stub mode).

**Steps**
1. `make demo-document` (registers a user, logs in, creates a profile,
   uploads a generated file, polls status, prints a download URL)
2. MinIO console (http://localhost:9001, minioadmin/minioadmin) →
   `documents-raw` bucket → today's object exists.
3. `make logs S=graphrag-service` — envelope received for the document,
   stub-mode processing note (real pipeline needs `OPENAI_API_KEY` +
   `requirements-graphrag.txt`, by design).
4. Watch: **GraphRAG processing** panel ticks.

**Expect:** script ends `E2E OK`; object in bucket; graphrag consumed and
logged a **stub result** ("GraphRAG pipeline unavailable" is correct without
LLM deps/keys); `document-processing` back to 0. **Known gap, expect it:**
document status stays `pending` forever — no worker→API status write-back
path exists yet (graphrag can't reach `api_db`), so the status endpoint only
ever reports the initial state. Finding recorded in CONCEPTUAL_REVIEW §12.

---

## EXP-12 · Cache outage (discovery experiment)

**Goal:** unlike the others, the expected result is *not* pre-verified —
stop Redis and **document what actually happens**. Discovery is a legitimate
experiment outcome; write it up.
**Validates:** your model of the cache layer vs its real failure mode.

**Steps**
1. Get a working token + profile (reuse `make demo-document` output or
   EXP-02's flow), confirm `GET /api/v1/profiles` 200s.
2. `docker compose stop redis`
3. Probe: repeat the GET and a create. Record: status codes? latency change?
   `make logs S=api-service` errors? `/ready` says redis not ok?
4. `docker compose start redis`; confirm recovery and note whether stale
   cache reappears (were keys lost? `docker compose exec redis redis-cli
   keys 'profile*'`).

**Write-up prompt:** should the API serve from Postgres when Redis is down
(cache-aside says yes)? Does it? File the finding in
`documentation/experiments/` — and into CONCEPTUAL_REVIEW if it contradicts
the design.

> **Answered 2026-07-10** (first exit run): fallback works, 0% errors, but
> requests silently pay 0.3–4 s dial penalties and only `/ready` reports the
> outage — see
> [the write-up](documentation/experiments/2026-07-10-cache-outage-latency-amplification.md).
> Future runs: treat that behavior as the calibrated Expect.

---

## Cluster experiments (v2 — the kind lab)

These run against `make cluster-up` instead of compose. Command mapping and
port-forward recipes: [deploy/k8s/README.md](deploy/k8s/README.md). The
compose Grafana panels don't see the cluster until v3 — watch via
`make cluster-queues`, `kubectl get`, and k6 output instead.

---

## EXP-20 · Catalog parity on kind

**Goal:** the v1 experiment catalog holds on the cluster — same behaviors,
kubectl levers instead of compose ones.
**Validates:** the entire v2 port: manifests, secrets, registry images,
ingress+TLS path, migrations, netpol allows (by everything working).

**Steps**
1. `make cluster-up` (single profile), then `make cluster-sim-smoke` —
   EXP-02 through the TLS ingress.
2. `make cluster-sim-burst` while watching `make cluster-queues` — EXP-04's
   climb-and-drain.
3. Worker outage (EXP-06): `kubectl -n lab-core scale deploy/email-worker
   --replicas=0`, flood 100 via the port-forwarded mgmt API (README recipe),
   confirm consumers=0 + backlog, scale back to 1, watch the drain.
4. Scale-out (EXP-07): `make cluster-scale S=email-worker N=3`, repeat the
   flood, compare drain slope; note the k8s Service now load-balances scrapes
   (the EXP-07 undercount argument resolves in v3 with pod-level discovery).
5. Poison (EXP-05): publish.py poison via port-forward; DLQ counts +6/+6/+6
   and +3; `make cluster-logs S=email-worker` shows both error flavors.
6. Broker outage (EXP-09): scale statefulset/rabbitmq to 0 with a persistent
   backlog waiting; probe degradation (CRUD 200 via ingress, task submit
   5xx, api pods NotReady w/ `/ready` rabbitmq down); scale back; backlog
   survives and drains.
7. Document E2E (EXP-11): README's demo-document variant against
   https://api.lab.local.

**Expect:** every referenced experiment's original Expect holds (calibration
numbers may shift with the extra ingress/CNI hops — record deltas); TLS is
lab-CA-issued end-to-end; nothing needs `docker compose` at any point.

---

## EXP-21 · Node kill & rescheduling (multinode)

**Goal:** lose a node under load; watch Kubernetes reschedule and the system
recover. The era-1 k6-analysis quality bar applies to the write-up.
**Validates:** multinode profile, PDBs, infra pinning (stores survive on the
infrastructure node), stateless service rescheduling, load continuity.

**Steps**
1. `make cluster-down && make cluster-up PROFILE=multinode`
2. `make cluster-sim-load SIM_DURATION=5m` in one terminal.
3. Find where api-service replicas run (`kubectl -n lab-core get pods -o
   wide`); pick the application node hosting one of them.
4. Drain it: `kubectl drain <node> --ignore-daemonsets
   --delete-emptydir-data` — watch the PDB hold (one api replica keeps
   serving) and pods reschedule to the surviving application node.
5. Then the blunt version: `docker stop <other-app-node-container>` (node
   *kill* from the cluster's view — the kubelet vanishes, nothing drains) —
   watch NotReady detection (~40s), pod eviction after the 5m default
   toleration or force-delete, k6 error blip vs the drain's zero blip.
6. Recover: `docker start` the node container / uncordon the drained one.

**Expect:** the graceful drain loses no requests (PDB + 2 api replicas);
the hard kill shows the detection→eviction latency honestly (write the
numbers down); stores never move off the infrastructure node; k6 finishes
with failure rate matching the kill window only.

---

## EXP-22 · Image path: change → push → rolling restart → rollback

**Goal:** the registry workflow end-to-end: a code change reaches the
cluster through localhost:5001, and rollback works by tag.
**Validates:** ADR-002.3 — real push/pull semantics, imagePullPolicy:
Always, tag-addressable rollback.

**Steps**
1. Baseline: note `kubectl -n lab-core get deploy api-service -o
   jsonpath='{.spec.template.spec.containers[0].image}'` and hit
   /health via the ingress.
2. Make a visible change (e.g. bump the /health payload's version string or
   a log line), `make images TAG=exp22`.
3. `kubectl -n lab-core set image deploy/api-service
   api-service=localhost:5001/api-service:exp22` — watch the rolling update
   (maxUnavailable 0: old pods serve until new ones are Ready).
4. Confirm the change is live through the ingress.
5. Rollback: `kubectl -n lab-core rollout undo deploy/api-service`, confirm
   the old behavior returns; `kubectl rollout history` shows both revisions.

**Expect:** zero failed requests during both rollouts (probe-gated);
the registry catalog (`curl localhost:5001/v2/_catalog`) lists all six
images; rollback is one command and sticks.

---

## EXP-23 · Zero-trust proof

**Goal:** the network policies actually deny what the diagram says they
deny — negative testing, not vibes.
**Validates:** default-deny + explicit allows (phase brief §7); that
kind ≥ v0.23 (kube-network-policies in kindnet) enforces NetworkPolicy.

**Steps**
1. Scratch pod: `kubectl -n lab-core run scratch --rm -it
   --image=busybox:1.36 --restart=Never -- sh`
2. Denied paths (each should time out, not connect):
   `nc -zv -w 3 postgres.lab-infra 5432` (scratch pod matches no allow),
   same for `redis.lab-infra 6379`, `rabbitmq.lab-infra 5672`.
3. Denied even for real workloads: `kubectl -n lab-core exec
   deploy/email-worker -- sh -c 'nc -zv -w 3 postgres.lab-infra 5432'` —
   workers may talk to rabbitmq only.
4. Allowed paths work: from the scratch pod DNS resolves (allow-dns), and
   `kubectl -n lab-core exec deploy/api-service -- wget -qO- -T 3
   http://auth-service:3000/health` returns 200.
5. Remove-and-restore — and learn the real semantics: deleting only the
   *ingress-side* guards (`lab-infra: default-deny-all postgres`) does NOT
   open the path (the worker's egress side still denies); deleting the
   worker-selecting egress policies (`lab-core: default-deny-all workers`)
   is *still* not enough — **`allow-dns`'s empty podSelector selects every
   pod, and any selecting egress policy imposes default-deny for everything
   it doesn't allow** (policies are a union of allows; there is no deny
   primitive). Only after `lab-core: allow-dns` is also gone does
   worker→postgres connect. Re-apply the overlay → denied again.

**Expect:** every deny is a timeout, every allow succeeds, and the flip
opens only when **no** egress policy selects the worker (calibrated
2026-07-10: exactly the allow-dns removal was the last domino) — proving
enforcement and teaching union-of-allows in one move. Cleanup: re-apply the
overlay (15 policies: 6 lab-core + 9 lab-infra).

---

## Adding an experiment

Copy this skeleton; keep Watch concrete (panel names, PromQL, commands) and
Expect falsifiable. When the Mission Control UI arrives (PRD v5), this
structure becomes the machine-read definition, so keep the headings.

```markdown
## EXP-NN · Title

**Goal:** one sentence — the operational question being asked.
**Validates:** which implementation claims this exercises.

**Steps**
1. ...

**Expect:** falsifiable observations.
**Cleanup:** ...
```
