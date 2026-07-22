# Cross-Service Contracts (api-publisher) — SKELETON

This is the doc pattern that kept the lab honest, cut down to what an API
publisher owns. It is the **single source of truth** for every surface where
this service touches another. Fill in the `<PLACEHOLDER>`s for your repo and
keep this file in the same change as any code that reshapes a surface below.

The publisher owns two outward contracts: what it **publishes** to the broker
(§1–2) and how it **authenticates** callers (§3). Everything else is internal.

## 1. Topology (routing keys → broker resources)

The API is a **publisher only**. It never declares topology — your broker
provisioning (e.g. a `definitions.json` loaded at boot; see
`deploy/rabbitmq/definitions.json`) owns exchanges, queues, bindings, and DLQ
args. The API verifies passively at connect and crashes if a resource is
missing. This table MUST match both `internal/task/tasktypes.go`
(`DefaultRoutingMap`) and the broker definitions.

| Routing key | Exchange (direct, durable) | Queue (durable) | Consumer |
|---|---|---|---|
| `example.task` | `example-tasks` | `example-processing` | `<your worker>` |
| `<add yours>` | `<exchange>` | `<queue>` | `<consumer>` |

DLQ convention (recommended): dead-letter exchange `<exchange>.dlx`, queue
`<queue>.dlq`. Consumer-side retry/idempotency is the consumer's contract —
see the `worker-go` template.

## 2. Envelope (every message, JSON)

Built at one place (`internal/task/envelope.go: BuildEnvelope`) and stored
whole in the outbox, so the relay publishes it verbatim. The shape is FROZEN —
consumers decode exactly this and MUST tolerate unknown extra fields and MUST
NOT require `metadata`:

```json
{
  "id": "uuid",
  "type": "<routing key>",
  "timestamp": "ISO-8601 UTC",
  "correlation_id": "uuid",
  "payload": { },
  "metadata": { "source": "api-publisher", "trace_id": "..." }
}
```

Per-task `payload` shapes: document one per task type (e.g. `ExamplePayload`
in `tasktypes.go`). Delivery is **at-least-once**: a crash between broker
publish and outbox mark-sent re-publishes the row, so consumers MUST dedupe on
`id` (idempotency — `worker-go`).

## 3. Auth contract (this API ← your auth service)

Access tokens are JWTs signed **RS256** with a `kid` header. The API verifies
them **locally** against the cached JWKS — no per-request hop to the auth
service (that would make it a request-path SPOF).

`GET {AUTH_URL}/.well-known/jwks.json` — **no auth, no rate limit**:
- 200: `{ "keys": [ { "kty":"RSA", "use":"sig", "alg":"RS256", "kid":"…", "n":"…", "e":"…" } ] }`
- The token header `kid` selects the key; an unknown `kid` triggers one
  rate-limited refresh (key-rotation path).

Claims consumed: `userId`, `email`, `role`, and `tokenType` (must be
`ACCESS_TOKEN`). Adjust the `Claims` struct in `internal/auth/jwks_verifier.go`
to match your issuer.

**Seams (not implemented here — wire if you need them):**
- **Revocation-strict / legacy algorithm:** local verify can't see
  revocations mid-token-lifetime and rejects non-RS256. If you need either,
  add an HTTP-introspection fallback (`POST {AUTH_URL}/…/validate`) in the
  middleware.
- **Dev bypass:** `AUTH_DISABLED=true` skips verification entirely (fixed dev
  user). Bootstrap-only — never in a real environment.

## 4. Environment variables

The API reads these (all have local-dev defaults; see `internal/config`):

`PORT`, `POSTGRES_DSN`, `RABBITMQ_HOSTS` (comma-separated `host:port`),
`RABBITMQ_USERNAME`, `RABBITMQ_PASSWORD`, `RABBITMQ_VHOST`, `AUTH_URL`,
`AUTH_TIMEOUT`, `AUTH_DISABLED` (dev only).

## 5. Health & observability

- `GET /healthz` — liveness + Postgres readiness (probes point here).
- `GET /metrics` — Prometheus. Outbox health lives here: `api_outbox_pending`
  (gauge — **alert if it grows**), `api_outbox_published_total`,
  `api_outbox_publish_errors_total`.
- Structured JSON logs; graceful shutdown on SIGTERM/SIGINT (drains HTTP, then
  stops the relay — unsent rows resume on next start).
