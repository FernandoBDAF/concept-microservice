# GraphRAG Service

Python RabbitMQ consumer that processes `document.process` tasks: downloads
documents from MinIO and, once the optional heavy pipeline is installed and
configured, runs GraphRAG ingestion + knowledge-graph extraction, storing
results in MongoDB.

## Status (as of 2026-07-07, refactor pass)

- Core consumer (RabbitMQ -> validate -> process -> ack), health/metrics
  server, and config are implemented and verified: `python3 -m compileall
  src cmd` passes (485 files), and the entrypoint imports and constructs
  cleanly with only `requirements.txt` installed -- no ML/LLM deps, no live
  infra required.
- The GraphRAG/ingestion pipeline (`src/domain/graphrag`, `src/domain/ingestion`,
  plus the large `src/graphrag/**` tree) is present but lazily imported and
  **unverified end-to-end** in this pass (no MongoDB/OpenAI available here).
  It activates automatically once `requirements-graphrag.txt` is installed
  and `OPENAI_API_KEY` is set; until then the worker logs a warning and
  stores a stub result (`{"status": "stubbed", ...}`) per message instead of
  crashing.
- No automated test suite exists. `tests/publish_test_message.py` is a
  manual script that publishes one `document.process` message to a live
  broker (matches `MESSAGE_FORMAT.md` exactly).

## Running

```bash
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt   # core only, no heavy ML deps
.venv/bin/python cmd/main.py
```

To activate the real GraphRAG pipeline:

```bash
.venv/bin/pip install -r requirements-graphrag.txt
export OPENAI_API_KEY=...
```

## Configuration (environment variables)

Contract-pinned (`graph-worker/shared/contracts/`, `CONTRACTS.md` section 4):

| Variable | Default | Notes |
|---|---|---|
| `RABBITMQ_URL` | unset | e.g. `amqp://guest:guest@rabbitmq:5672/`. Takes precedence over the `RABBITMQ_HOST`/`PORT`/`USER`/`PASSWORD`/`VHOST` fields below when set. |
| `MONGODB_URI` | `mongodb://admin:password@mongodb:27017` | matches local-dev compose credentials |
| `MINIO_ENDPOINT` | `minio:9000` | |
| `MINIO_ACCESS_KEY` | `minioadmin` | matches local-dev compose credentials |
| `MINIO_SECRET_KEY` | `minioadmin` | matches local-dev compose credentials |
| `HEALTH_PORT` | `8080` | serves `GET /health`, `/ready`, `/live` |

Extra (pre-existing, not contract-pinned; kept with sane defaults):

| Variable | Default | Notes |
|---|---|---|
| `RABBITMQ_HOST` / `PORT` / `USER` / `PASSWORD` / `VHOST` | `rabbitmq` / `5672` / `guest` / `guest` / `/` | used to build a connection URL only when `RABBITMQ_URL` is unset |
| `MONGODB_DATABASE` | `graphrag` | |
| `OPENAI_API_KEY` | `` (empty) | required only to activate the real pipeline; empty = stub mode |
| `MINIO_USE_SSL` | `false` | |
| `METRICS_PORT` | `8081` | Prometheus `/metrics` |
| `LOG_LEVEL` | `INFO` | |

## RabbitMQ contract

Consumes routing key `document.process` from durable direct exchange
`document-tasks`, queue `document-processing`, **prefetch 1**. Topology
(exchange/DLX/queue/DLQ args, 12h queue TTL, 7-day DLQ TTL) is declared to
match the publisher exactly -- see
`api-service/internal/infrastructure/rabbitmq/client.go` (`ensureTopology`)
and `api-service/internal/domain/task/model.go`
(`DefaultRoutingMap["document.process"]`). Envelope and payload shape:
`graph-worker/shared/contracts/MESSAGE_FORMAT.md`. Unknown extra fields and
a missing `metadata` block are tolerated.

Poison/failed messages are nacked without requeue, which routes them to
`document-processing.dlq` via the configured dead-letter exchange, and a
single bad message no longer takes down the consumer loop (see
`src/worker/consumer.py::_handle_delivery`).

## Known gaps / leftovers

- `src/graphrag/**` is a near-complete duplicate of `src/{app,core,domain,
  infrastructure,lib}/**` (identical filenames, nested under `src/graphrag/`).
  The worker imports from the top-level `src.domain`/`src.core` paths; the
  `src/graphrag/` copy looks unused by the consumer path and is a candidate
  for deletion -- not removed in this pass (large blast radius, out of the
  time budget).
- The GraphRAG/ingestion pipeline itself has not been executed even once
  here; only import-time and static checks were performed against it.
- No automated test suite.
