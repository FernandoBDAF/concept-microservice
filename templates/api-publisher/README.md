# Template: API publisher (Go)

**An API that publishes work reliably** — and nothing more. A caller POSTs a
task; the API writes it to a **transactional outbox** in the same Postgres
transaction, returns `202`, and an in-process **relay** publishes it to
RabbitMQ with broker confirms. If the process dies between the DB commit and
the publish, the row is still there and is published on restart: the
crash-window is closed by construction, and delivery is **at-least-once**
(consumers dedupe on envelope `id`). Access is gated by **local JWKS
verification** — no per-request hop to the auth service.

Module: `example.com/api-publisher`. Copy-then-own (no shared library, no
semver) — see [GRADUATION.md](../GRADUATION.md).

## What's inside

| Piece | Path | What it is |
|---|---|---|
| Outbox + relay | `internal/outbox/` | the table's Go side: store, `PendingBatch … FOR UPDATE SKIP LOCKED`, `Relay` loop |
| Task submission | `internal/task/` | one envelope construction point, the routing map, `Service.Submit` |
| **Your task types** | `internal/task/tasktypes.go` | the one file you edit to define tasks |
| Auth middleware | `internal/auth/`, `internal/httpapi/middleware.go` | JWKS verifier + local-verify middleware |
| HTTP server | `internal/httpapi/` | `POST /tasks`, `GET /healthz`, `GET /metrics` (stdlib `net/http`) |
| Broker/DB wiring | `internal/rabbitmq/`, `internal/postgres/` | confirm-mode publisher, passive topology verify, pooled DB |
| Migration | `migrations/000001_create_outbox.*.sql` | the `outbox` table + partial pending index |
| Contracts | [CONTRACTS.md](CONTRACTS.md) | topology, envelope, auth — the surfaces you own |
| Deploy | `compose.snippet.yml`, `deploy/k8s/base/` | local stack + kustomize base |
| Smoke | `test/bootstrap.sh` | proves publish + crash-window on a fresh copy |

## How to adapt (≈ 3 steps)

1. **Copy & rename.** Copy this directory; set your module path in `go.mod`
   (replace `example.com/api-publisher`) and run `go mod tidy`. Then **edit
   `internal/task/tasktypes.go`** — the single adapt point: for each task type
   give it a routing key, its broker exchange/queue in `DefaultRoutingMap`, and
   (recommended) a typed payload + a typed `Submit…` helper. Mirror those rows
   into [CONTRACTS.md](CONTRACTS.md) §1–2 and into your broker topology
   (`deploy/rabbitmq/definitions.json` here — the API verifies it passively and
   refuses to start if a resource is missing).

2. **Wire your writes through the outbox.** For fire-and-forget submissions,
   `POST /tasks` already does the right thing. For a task that must be
   consistent with a domain write, open one transaction and call
   `outbox.Store.Add(ctx, tx, routingKey, envelope)` alongside your `INSERT` so
   the domain row and its task event commit together (build the envelope with
   `task.BuildEnvelope` so every path produces the same shape). Point
   `AUTH_URL` at your auth service; adjust the `Claims` struct in
   `internal/auth/jwks_verifier.go` to your issuer's claims.

3. **Smoke it.** `test/bootstrap.sh`: brings up `compose.snippet.yml`, POSTs a
   task, asserts the outbox row exists and the relay publishes it (visible via
   the RabbitMQ management API), then kills the relay between commit and publish
   and asserts the orphaned row publishes on restart. Break the relay or the
   routing map and it fails — that's the regression harness (EXP-81).

## Run it locally

```sh
go build ./... && go vet ./... && go test ./...   # unit suite (no Docker)
./test/bootstrap.sh                                # full smoke (needs Docker, curl, jq)
```

`compose.snippet.yml` sets `AUTH_DISABLED=true` so the smoke runs without an
auth service. **That is dev-only** — never set it anywhere real; the app logs a
loud warning when it's on.

## What was trimmed (and why it's not here)

This template is the **publish path only**. Deliberately left out, with the
seam documented rather than shipped half-done:

- **Task-results consumption / status lifecycle.** In the lab the API also
  consumed a `task-results` queue to advance document status. That is
  *consume-side* and domain-specific — it belongs with the consumer
  (`worker-go` template) or in your own status updater. Wire it there.
- **Domain stores (documents/profiles), object storage (MinIO), Redis cache,
  distributed tracing (OpenTelemetry), per-request auth introspection, viper.**
  None are on the publish path. Auth here is local JWKS verify only; if you
  need revocation-strict checks or a legacy token algorithm, add an
  introspection fallback in the middleware (CONTRACTS.md §3).

## Proven by (honest status)

Every pattern here traces to a lab experiment. Per the v4 deferral ledger,
these were **authored but their live runs are still pending** — cited honestly:

- **EXP-42** — outbox crash-consistency (commit-then-publish crash window
  closed). *Authored; live run pending.* The unit tests
  (`internal/outbox/outbox_test.go`) pin the SQL shape and the batch
  select→publish→mark transaction; `test/bootstrap.sh` B is the live proof once
  run.
- **EXP-43** — local verify under an auth-service outage (JWKS cache serves
  while the issuer is down). *Authored; live run pending.*
- **EXP-45** — the smoke as the consuming repo's CI seed. *Authored; live run
  pending.*

The unit suite that ships green today: the outbox store/relay tests and the
JWKS verifier tests (adapted from the lab source), plus task-service and
handler whitelist tests.
