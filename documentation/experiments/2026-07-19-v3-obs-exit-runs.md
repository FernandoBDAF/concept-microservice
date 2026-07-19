# v3 exit runs — EXP-30..34 on the kind cluster + obs stack

**Date:** 2026-07-19 · **Runner:** Claude (deferred-validation session; the
v3 implementation itself was the 2026-07-19 expedited pass) · **Cluster:**
kind `single` (kindest/node:v1.36.1, kind v0.32.0) on a fresh Windows 11 +
WSL2 host — the toolchain bootstrap that machine needed is PR'd separately
(README Dependencies guide) · **Result:** all five pass; **six real
defects** found and fixed en route (four of them the exact class the
deferred ledger existed to catch: code that compiles green but had never
executed).

Method note: every "UI step" below was also executed as its API equivalent
(same query, same panel expression) so the evidence is reproducible from a
terminal; where a drill's Expect didn't survive contact with reality, the
calibration is recorded rather than papered over — same convention as the
v1.1/v2 runs.

## Defects found by the runs (all fixed, committed on this branch)

1. **Image builds were impossible** — v3's OTel work auto-bumped `go.mod`
   to 1.25.0 (api-service, operational-workers) while every Dockerfile
   pinned `golang:1.24-alpine` (GOTOOLCHAIN=local): `make images` failed on
   the first line of the first deferred run. `make verify` never sees it —
   host Go auto-downloads the newer toolchain. Fix: `golang:1.25-alpine`.
2. **Every Go service crash-looped at startup on kind** —
   `resource.Merge(resource.Default(), NewWithAttributes(semconv/v1.26.0…))`
   returns `ErrSchemaURLConflict` under SDK v1.44 (default resource schema
   1.41.0). Deterministic fatal at init, *only* when
   `OTEL_EXPORTER_OTLP_ENDPOINT` is set — the unit suites only ever ran the
   no-endpoint path. 12 crash-restarts per pod before the fixed image
   landed; `imagePullPolicy: Always` + crash-backoff re-pull then healed
   the deployment without a rollout. Fix: merge a schemaless resource
   (adopts the SDK's schema; survives future bumps) in both duplicated
   `tracing.go` copies + regression tests that exercise the endpoint path.
3. **The lab-logs index template raced fluent-bit and lost** — both wait on
   OpenSearch readiness; fluent-bit's first bulk won and created the index
   with guessed mappings: zap's epoch-float `ts` typed the field `float`,
   every ISO-`ts` doc (graphrag, opensearch's own logs) was rejected with
   `mapper_parsing_exception`, and fluent-bit's failed chunks retried then
   dropped — taking co-batched lines with them (the EXP-31 email-worker
   error lines among the casualties). Fix: template maps `ts: keyword`
   (`@timestamp` is the time field) and the Job is now convergent — after
   the PUT it deletes any pre-template `lab-logs-*` index (disposable by
   ADR-003.3), so the race can't leave a poisoned index behind.
4. **Worker DLQ error logs had no `trace_id`** — EXPERIMENTS.md and
   OBSERVABILITY.md document the triage pivot as "the error lines carry
   the envelope trace_id"; the handler-error line didn't. Fix: log the
   consume span's trace id (always valid at that site — extracted parent
   or fresh root). The unmarshal-flavor line stays bare: a non-JSON body
   has no envelope to extract, and its triage path is queue+level+payload.
5. **Prometheus could not scrape guest namespaces** — the guest-side netpol
   admitted lab-obs, but lab-obs's `prometheus-scrape-egress` had never
   learned about guest namespaces; zero-trust cuts both ends. Fix: one
   egress rule selecting `lab.local/tier: guest` namespaces on the
   contract's :8080 — every future guest inherits scrapeability from the
   namespace label (full story in EXP-34).
6. **EXP-HG-01's compose crash lever could never fire** — `docker kill` is
   a manual stop, which `restart: unless-stopped` deliberately does not
   revive; the drill claimed self-healing with a lever that cannot show
   it. Fix: crash from inside (`docker exec … kill 1`), validated live —
   revived in ~6 s (details in EXP-34).

Cross-platform verify fixes that fell out of running the battery on
Windows (not lab defects): `rimraf --glob` in auth-service scripts,
`*.exe` gitignore.

## EXP-30 · One trace, whole system — ✅

`make demo-document` variant through the TLS ingress
(`AUTH_URL=https://auth.lab.local API_URL=https://api.lab.local
CURL_OPTS=-k`): register → login → profile → upload → status ×5 → download
URL, all green. The upload log line's `trace_id` is a real 32-hex W3C id;
Tempo (queried via the Grafana datasource proxy, uid `tempo`) returns one
trace for it:

| Service | Spans (18 total) |
|---|---|
| api-service | `HTTP POST` → `POST /api/v1/documents/upload` → `sql.conn.query` → `publish document.process` |
| auth-service | `POST /v1/auth` (introspection) + express middleware chain + `pg-pool.connect`, `pg.query:SELECT auth_db` |
| graphrag-service | `consume document-processing` |

**3 distinct services** in one trace (assert ≥3 ✓); graphrag's JSON logs
carry the same trace_id (5 correlated lines ✓) — envelope
`metadata.trace_id` = real propagated context (decorative no more), the
core ADR-003.2 claim.

## EXP-31 · Log triage — ✅ (after defects #3/#4; two calibrations)

Poison batch (`publish.py poison --count 3` through the port-forwarded
mgmt API): 24 sent, DLQ panel query
(`rabbitmq_queue_messages{queue=~".+\\.dlq"}`) stepped **+21 —
email/image/profile +6 each, document +3** (graphrag ACK-drops the
valid-envelope flavor by design; exact EXP-05 calibration).

Triage loop exactly as documented (Discover filters ==
`kubernetes.container_name: email-worker AND level: error`, last 10m):
**6 hits** — 3 validation errors (`invalid recipient email`) now carrying
`trace_id`, 3 unmarshal errors (`invalid character 'P'…`) without.
Pivoting a validation error's trace_id returns its full (single-service)
story; the raw payloads were recovered from `email-processing.dlq` head
with `x-first-death-reason: rejected` — both flavors identified without
touching kubectl.

Calibrations recorded against the drill text:
- publish.py poison **bypasses HTTP**, so its trace_ids are per-message
  and single-consumer — the "≥2 services correlated" expectation belongs
  to API-originated traffic. Proven there: the document-upload trace_id
  returns lines from **api-service + graphrag-service** in one OpenSearch
  query.
- The non-JSON flavor cannot carry a trace_id (no envelope to extract);
  its triage path is queue+level+DLQ payload, which works.

## EXP-32 · Page yourself — ✅

SSE subscriber on `https://ntfy.lab.local/lab-alerts/sse` (through the
ingress) armed **before** the incident; then `email-worker → 0 replicas`,
`flood --routing-key email.send --count 600` (routed in 8 s).

| Clock (UTC) | Event |
|---|---|
| 20:25:58 | flood done, consumers absent |
| **20:31:41** | **push arrives over SSE**: `QueueDepthSustained [firing]`, priority `high`, tag `warning`, full description + runbook link → EXP-04 |
| 20:31:5x | only *after* the push: rule state corroborated via API, worker scaled back to 1 |
| 20:33:30 | 600 → 0 drained (~7 msg/s single replica) |
| **20:33:41** | `QueueDepthSustained [resolved]` push (priority default, ✅ tag) — no manual silencing |

5m43s flood→page = `for: 5m` + scrape/eval + Alertmanager `group_wait
15s`, as designed. The severity→priority mapping (warning→high,
resolved→default) worked verbatim. Three `DLQGrowth [resolved]` pushes
from EXP-31's batches rode the same group interval — the rest of the
alert set is demonstrably live, not just the drilled rule.

## EXP-33 · SLO baseline — ✅

10 VUs / 10 m steady through the TLS ingress (`TARGET=cluster`), DLQs
purged first: **4573 iterations, 0 interrupted**. Snapshot (full block
appended to SLO-BASELINE.md):

| Measure | Baseline |
|---|---|
| API p50 / p95 / p99 | 18 ms / 220 ms / **966 ms** |
| Request rate | 30.8 req/s |
| 5xx ratio | **0** (no 5xx series in the window) |
| Email drain rate | 7.6 msg/s (1 replica — matches EXP-32's observed drain) |
| Peak work-queue depth | 6 (consumers kept pace) |
| Memory envelope (lab-core) | auth 94 MB · graphrag 53 MB · api 20–23 MB/replica · workers 11–21 MB |

Encoded (`prometheusrule.yaml`, applied and verified loaded):
`APIP99OverSLO` 0.5 s placeholder → **3 s** (baseline ×3); new
`APIAvailabilityBelowSLO` (5xx ratio > 0.005 for 5 m = 99.5% SLO);
`QueueDepthSustained` **kept at 500** — the worksheet's peak-×2 formula
yields 12, which would page on every intentional flood drill; the
deviation is documented in the worksheet.

Two operational notes from the run itself:
- The in-script `kubectl port-forward` died during the 10-minute window
  (the v2 write-up warned exactly this), truncating the snapshot append;
  repaired by re-evaluating the same instant queries **pinned to the run
  window** (`&time=20:57:50Z`, `[10m]` ranges) — numerically equivalent,
  and the truncated block was replaced, not left to lie.
- Minutes after the run, a host-side CPU stall (Windows installer churn +
  antivirus on the shared WSL2 cores) blew 1 s probe deadlines across
  four unrelated pods at once; Prometheus's liveness probe killed it into
  a clean restart (graceful TSDB shutdown, WAL replay, ~1 min metrics
  gap), `ScrapeTargetDown` went pending and cleared. Laptop-cluster node
  pressure behaving exactly as ADR-003.3 predicted — observed, not
  suffered.

## EXP-34 · Hello-guest onboarding — ✅ (and the run found defect #5)

**kind flavor.** `kubectl apply -k k8s/base` + `k8s/obs` (images were
already in the registry from `make images`): rollouts in seconds;
`/health` `/ready` 200 via port-forward; `https://hello-guest.lab.local/`
answers through the shared ingress with the lab-CA cert. Prometheus
discovered both guest ServiceMonitor targets — **and could not scrape
them** (`down`, context deadline): the guest-side netpol admitted
lab-obs, but lab-obs's `prometheus-scrape-egress` had never learned about
guest namespaces. Zero-trust cuts both ends. Fix: one egress rule
selecting `lab.local/tier: guest` namespaces on the contract's :8080 —
every future guest now inherits scrapeability from the namespace label.
After the fix: both targets `up` (empty lastError), a 2000-request burst
plots at **44.8 req/s** on `rate(hello_guest_web_requests_total[1m])` in
the shared Prometheus.

Isolation proof, from the *real* guest workload (no scratch pod):
`kubectl exec deploy/hello-guest-worker -- nc -zv -w 3 …` →
`postgres.lab-infra:5432` **timeout**, `rabbitmq.lab-infra:5672`
**timeout** (nc resolving the FQDNs doubles as the DNS-allowed control).
Self-heal: worker pod deleted → replacement Running in ≤25 s,
`hello_guest_jobs_total` restarted from a low value — the counter reset
`rate()` is built to absorb.

Calibration note: 3000 sequential TLS handshakes through the ingress
crawl at single-digit req/s from WSL — the burst step wants connection
reuse (curl URL-globbing `?i=[1-500]` ×4 parallel, ~20 s). EXPERIMENTS.md
EXP-HG-01's `for` loop works but budget minutes, not seconds.

**compose flavor.** First `make up` on this machine surfaced a
Windows-Docker-Desktop environment quirk, not a repo defect: the fresh
`rabbitmqdata` named volume received the image's `/var/lib/rabbitmq` with
`.erlang.cookie` owned root:root (containerd-snapshotter copy-up), and the
broker died on `eacces` — one `chown 100:101` on the cookie and the stack
came up healthy (14 containers). Then the guest, by the book: own compose
project on the 41xx block; `/` `/health` `/ready` `/metrics` all answer;
the **host** Prometheus lists both guest scrape URLs; a 2000-request burst
plots at **44.7 req/s** host-side (same number as the kind flavor — tidy).
Teardown leaves zero residue.

The run also caught **defect #6, in the drill itself**: EXP-HG-01 said
`docker kill` + `restart: unless-stopped` proves compose self-healing —
but Docker treats kill/stop as *manual* stops and `unless-stopped`
deliberately does not revive them; the worker stayed down. The correct
crash lever is `docker exec hello-guest-worker-1 kill 1` (a failure from
inside): validated — revived in ~6 s, `RestartCount=1`, fresh `StartedAt`.
Drill text corrected.

## Status page + watch-without-terminal spot-check

**Watch spot-check (three drills, UIs only).** Lab Overview is provisioned
in the kind Grafana (sidecar ConfigMap) with panels: API request rate ·
API latency p50/p95/p99 · Queue depth (work queues) · Dead-letter queues ·
Worker throughput · Worker errors · Auth request rate · GraphRAG
processing · Recent traces · Trace workflow. Against live data:

| Drill | Watch step | UI answer |
|---|---|---|
| EXP-04 burst | queue depth + drain rate | "Queue depth (work queues)" (4 series live) + "Worker throughput" (live) |
| EXP-05/31 poison | DLQ growth | "Dead-letter queues" (4 series live; stepped +21 during EXP-31) |
| EXP-06 outage | consumers gone, backlog, p99 | `rabbitmq_queue_consumers` (live), depth panel, "API latency" p99 (live) |

kube-prometheus-stack's node dashboards (Nodes / USE Method / Compute
Resources) cover the node-pressure watch; Grafana, OpenSearch Dashboards
and Prometheus UIs all answer 200 through their ingresses. Every Watch
step of the three drills resolves to a live panel — no terminal required.

**Status page (live-truth, both targets).** controld on Windows (exported
kubeconfig; shells out to the same docker/kubectl/kind an operator uses):
`/api/targets` → compose **true** + kind **true**; compose status mirrors
`docker compose ps` service-for-service (running/healthy); kind status
aggregates 29 workloads across lab-core/lab-infra/lab-obs; health probes
return real latencies (api 35 ms, auth 76 ms, graphrag 48 ms). The Next.js
page renders the terminal-style console with both target chips green,
14 service cards with state/health badges, probes, links, and the
read-only banner — screenshot in the session evidence. One environment
note: Next.js 15 enforces Node ≥ 18.18 — the host's Node 18.17 refused to
even start the dev server (ZIP-installed Node 24 resolved it; the README
dependency guide's "Node 20+" line is the requirement to trust).

## Operational notes for future runs

- First `make obs-up` on a cold image cache exceeds the 300 s OpenSearch
  statefulset wait (the server image alone is the long pull) — the target
  is idempotent; the second run converges. Not worth a longer timeout:
  every warm-cache run fits easily.
- The obs helm pins all resolved and pulled (kps 87.17.0 → operator
  v0.92.1/Prometheus v3.13.1/Grafana 13.1.0, tempo 1.24.4 → 2.9.0,
  opensearch/dashboards 2.19.1, fluent-bit 3.2.10, ntfy v2.11.0) — the
  HANDOFF's "unverified pins" flag is closed.
- `kubectl logs --all-pods=true` prefixes every line with `[pod/…]` —
  strip before parsing the JSON.
- ntfy SSE through ingress-nginx works unbuffered (keepalives ~45 s beat
  the 60 s idle timeout); a scratch-topic plumbing check before the drill
  avoids debugging two things at once.
