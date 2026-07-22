# Template: async worker (Go)

A copy-ready AMQP worker: a hardened consume loop with reconnect+backoff,
retry-tier republishing, a SETNX idempotency guard, a results-publishing loop,
health/metrics server, and one example processor you swap for your own. Module
path is `example.com/worker` — the adapt step renames it.

Extracted from the lab's `graph-worker/operational-workers/` common core
(post-v4). Copy-then-own: no shared library, no semver
([GRADUATION.md](../GRADUATION.md)).

## What's inside

```
worker-go/
  cmd/example-worker/main.go        wiring: topology + processor → BaseWorker
  internal/common/queue/            envelope, consume loop (reconnect+backoff),
                                    retry-tier + DLX publishing, passive topology
                                    verify, results loop, publisher, metrics
  internal/common/idempotency/      SETNX guard (Redis prod / in-memory dev)
  internal/common/base/             worker lifecycle + health/metrics HTTP server
  internal/common/processors/       MessageProcessor interface
  internal/common/tracing/          OpenTelemetry bootstrap (off unless configured)
  internal/common/utils/            env + prometheus helpers
  internal/processors/example/      the one reference processor (replace this)
  scripts/generate-definitions.py   topology fragment generator for a new queue
  Dockerfile  compose.snippet.yml   container + rabbitmq+redis+worker stack
  deploy/k8s/                       minimal kustomize base (probes/limits/labels)
  deploy/rabbitmq/rabbitmq.conf     load_definitions at boot
  chart/                            the one Helm chart (ADR-002.1 exercise)
  test/bootstrap.sh                 compose smoke: 10 consumed, 1 poison → DLQ
  .github/workflows/ci.yml          build + vet + test + topology lint
```

## How it works (the patterns you're copying)

- **Broker-owned topology (ADR-008.4).** Exchanges/queues/bindings live in
  `definitions.json`, loaded by the broker at boot. Services never declare —
  the consumer *verifies passively* (a passive declare 404s loud if the
  topology is missing). Generate the file with the script below.
- **Retry tiers, not nack-requeue (ADR-008.1).** On a retryable handler error
  the consumer republishes the body to `<exchange>.retry` with routing key
  `<rk>.retry.<tier>` (tiers `5s → 30s → 2m`, chosen by the x-death count) and
  acks. After the last tier — or on an `ErrUnretryable` error — it publishes to
  `<exchange>.dlx` (the DLQ, poison path) and acks. It never nack-requeues on
  the normal path.
- **Idempotency guard (ADR-008.2).** First delivery of an `(envelope-id,
  attempt)` wins via Redis `SETNX`; a genuine duplicate is acked without
  reprocessing. The key is scoped by attempt so an *intentional* retry is still
  reprocessed. Guard errors **fail open** — dedupe is a net, not a lock.
- **Results loop (ADR-008.3).** After ack-worthy processing the worker publishes
  a `task.result` (`completed`/`failed`) to the shared `task-results` exchange —
  best-effort, never influencing the work-message routing.

## How to adapt (target: < 1 hour of the < 1 day budget)

1. **Copy + rename.** Copy this directory into your repo. Rename the module and
   generate your queue's topology:
   ```bash
   go mod edit -module github.com/you/yourworker   # then fix imports (sed)
   mv cmd/example-worker cmd/yourthing-worker
   python3 scripts/generate-definitions.py --pipeline yourthing:yourthing.task \
       --full -o deploy/rabbitmq/definitions.json
   ```
   Update the `WorkerConfig` in `cmd/*/main.go` (WorkerType/QueueName/
   ExchangeName/RoutingKey) to match the names you generated.

2. **Replace the processor.** Implement `processors.MessageProcessor`
   (`Process`/`Validate`/`Type`/`HandleError`) for your payload — swap the body
   of `internal/processors/example/`. Keep the shape: `Validate` is the cheap
   unretryable pre-check (a failure is routed straight to the DLQ); return a
   plain error from `Process` for a retryable failure, or wrap it with
   `queue.ErrUnretryable` to skip the retry tiers.

3. **Green the bootstrap.** `go build ./... && go vet ./... && go test ./...`,
   then `test/bootstrap.sh` (compose smoke: build, publish 10 → all consumed,
   publish 1 poison → 1 in DLQ).

## Fragment generator

`scripts/generate-definitions.py` emits the broker topology for a new queue with
the exact retry-tier + DLQ queue-argument conventions the consumer depends on
(these are destructive to change once messages are in flight — do not hand-edit
them). `--pipeline NAME:RK` derives exchange `NAME-tasks`, queue
`NAME-processing`, consumer `NAME-worker`:

```bash
# mergeable {exchanges,queues,bindings} fragment → stdout
python3 scripts/generate-definitions.py --pipeline example:example.task

# complete, load_definitions-ready definitions.json (adds vhost + guest user)
python3 scripts/generate-definitions.py --pipeline example:example.task \
    --full -o deploy/rabbitmq/definitions.json
```

Repeat `--pipeline` for several queues; the shared `task-results` loop is added
once. The retry tiers (`5s/30s/2m`) are kept in lockstep with
`internal/common/queue/retry.go` — change both or neither.

## Configuration surface (env)

| Var | Purpose | Default |
|---|---|---|
| `RABBITMQ_URL` | broker URL (authoritative) | `amqp://guest:guest@rabbitmq:5672/` |
| `RABBITMQ_HOST/PORT/USER/PASSWORD/VHOST` | finer overrides when `RABBITMQ_URL` unset | host `rabbitmq`, port `5672`, guest/guest |
| `REDIS_ADDR` | idempotency guard backend (`host:port`) | empty → **in-memory guard, dev only** |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP/http collector | empty → tracing off (no-op provider) |
| `OTEL_SERVICE_NAME` | trace service name | `<worker-type>-worker` |
| `HEALTH_PORT` | health/metrics HTTP port | `8080` |
| `FAIL_FIRST_N_ATTEMPTS` | test hook: fail first N attempts then recover | `0` (inert) |

HTTP endpoints: `/health`, `/ready`, `/metrics`.

## Tracing

**Included.** `internal/common/tracing/` is kept because it stays low-coupling —
only `base/worker.go` calls `tracing.Init`, and the consume loop already uses the
OTel API to continue the publisher's trace across the queue hop. It compiles and
unit-tests standalone, and is a **no-op unless `OTEL_EXPORTER_OTLP_ENDPOINT` is
set** (propagators are still registered so `traceparent` extraction works). If
you don't want OTel at all, delete the `tracing/` package and the `tracing.Init`
call in `base/worker.go` — that is the only seam.

## Proven by

Every pattern here carries the experiment that beat on it. Live-run status is
stated honestly:

| Pattern | Experiment | Status |
|---|---|---|
| Durable delivery + DLQ on poison | EXP-04 / EXP-05 / EXP-06 | real run history (durability + DLQ) |
| Retry tiers (`5s/30s/2m`, x-death routing) | EXP-40 | authored; live run pending (v4 deferral ledger) |
| SETNX idempotency guard (attempt-scoped) | EXP-41 | authored; live run pending |
| `task-results` completion/failure loop | EXP-42 | authored; live run pending |

The unit tests for retry routing (`queue/retry_test.go`,
`queue/results_test.go`), idempotency (`idempotency/guard_test.go`), and trace
propagation (`queue/trace_test.go`) pass today; the "live run pending" note
means the end-to-end behavior on a real broker has not yet been recorded in an
EXP-40..42 write-up, only authored and unit-proven.

## Bootstrap test contract (v8-HANDOFF §5)

`test/bootstrap.sh`: generate `definitions.json` → `docker compose up`
(rabbitmq + redis + worker) → publish 10 valid messages (assert the work queue
drains) → publish 1 poison message with a missing required payload field (assert
exactly 1 lands in `example-processing.dlq`, via the management API) →
`compose down`. Defensive waits/retries throughout. **Deliberately breakable
(EXP-81):** break the processor, the routing keys, or the generated topology and
the assertions fail. Needs a Docker daemon; not run by the in-template CI.
