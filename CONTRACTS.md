# Cross-Service Contracts (PINNED)

This file is the **single source of truth** for every surface where services touch.
No service may rename or reshape anything below without updating this file, the
root `docker-compose.yml`, and every consumer in the same change.

Canonical detail lives in:
- `graph-worker/shared/contracts/ROUTING_KEYS.md` (queues/exchanges)
- `graph-worker/shared/contracts/MESSAGE_FORMAT.md` (message envelope + payloads)

## 1. Service topology (docker-compose hostnames & ports)

| Service | Compose hostname | Container port | Host port | Notes |
|---|---|---|---|---|
| PostgreSQL 15 | `postgres` | 5432 | 5432 | DBs: `api_db`, `auth_db` (init script) |
| Redis 7 | `redis` | 6379 | 6379 | no password (local dev) |
| RabbitMQ 3.12 | `rabbitmq` | 5672 / 15672 | 5672 / 15672 | guest/guest, vhost `/` |
| MongoDB 7 | `mongodb` | 27017 | 27017 | admin/password |
| MinIO | `minio` | 9000 / 9001 | 9000 / 9001 | minioadmin/minioadmin |
| auth-service | `auth-service` | 3000 (`PORT=3000`) | 3000 | Node/Express |
| api-service | `api-service` | 8080 (metrics 8081) | 8080 / 8081 | Go/Gin |
| graphrag-service | `graphrag-service` | 8080 health, 8081 metrics | 8082 | Python consumer |
| email/image/profile workers | `<name>-worker` | 8080 health+metrics | none | Go consumers |
| Prometheus | `prometheus` | 9090 | 9090 | scrapes everything below |
| Grafana | `grafana` | 3000 | 3001 | admin/admin, Lab Overview dashboard |

Local-dev credentials (compose only, never production):
- Postgres superuser: `postgres`/`postgres`. `auth_db` owned by `auth_user`/`auth_password`.
- api-service DSN: `postgres://postgres:postgres@postgres:5432/api_db?sslmode=disable`
- Mongo URI: `mongodb://admin:password@mongodb:27017`
- JWT secret (dev): `local-dev-jwt-secret-change-me-32chars!!`
- MinIO bucket: `documents-raw` (api-service ensures it exists on startup)

## 2. RabbitMQ contract (publisher: api-service)

| Routing key | Exchange (direct, durable) | Queue (durable + DLQ) | Consumer |
|---|---|---|---|
| `document.process` | `document-tasks` | `document-processing` | graphrag-service |
| `email.send` | `email-tasks` | `email-processing` | email-worker |
| `image.process` | `image-tasks` | `image-processing` | image-worker |
| `profile.task` | `profile-tasks` | `profile-processing` | profile-worker |

DLQ convention: dead-letter exchange `<exchange>.dlx`, queue `<queue>.dlq`.

Envelope (every message, JSON):
```json
{
  "id": "uuid",
  "type": "<routing key>",
  "timestamp": "ISO-8601 UTC",
  "payload": { },
  "metadata": { "source": "api-service", "trace_id": "..." }
}
```
Payload shapes per task type: see `MESSAGE_FORMAT.md`. Consumers MUST tolerate
unknown extra fields (forward compatibility) and MUST NOT require `metadata`.

## 3. Auth HTTP contract (api-service → auth-service)

**Token model (ADR-009.1/.2):** access + refresh tokens are JWTs signed **RS256**
with a `kid` header when an auth-service keypair is configured (env
`JWT_PRIVATE_KEY` / `JWT_PUBLIC_KEY` = base64-encoded PEM single line, or raw PEM;
`JWT_ALGORITHM=RS256|HS256`, default RS256 when both keys are present). **HS256
stays the keyless fallback** and both algorithms are accepted during migration,
so compose/CI without keys keep working. Claims are unchanged and still embed
`userId`, `email`, `role`. Access tokens are stateless, TTL ≤15m; refresh tokens
are one-time-use, rotated against a DB `sessions` table (reuse ⇒ session revoked
+ audit). api-service verifies access tokens **locally** against the cached JWKS;
the HTTP introspection endpoint below remains available for revocation-strict
routes (opt-in).

### JWKS — public key distribution

`GET {AUTH_URL}/.well-known/jwks.json` — **no auth, no rate limit**.
- 200 response: `{ "keys": [ { "kty":"RSA", "use":"sig", "alg":"RS256", "kid":"…", "n":"…", "e":"…" } ] }`
- `kid` = first 8 bytes of SHA-256 over the SPKI DER (hex); it matches the token header `kid`.
- Empty `keys` array in the HS256 keyless fallback.

### Introspection — still available (opt-in)

`POST {AUTH_URL}/v1/auth/token/validate`
- Headers: `Authorization: Bearer <token>`, `Content-Type: application/json`
- Body: `{"token": "<jwt>"}`
- 200 response (shape is FROZEN — api-service decodes exactly this):
```json
{
  "status": "success",
  "message": "...",
  "data": { "valid": true, "user": { "id": "...", "email": "...", "role": "..." } }
}
```
- On invalid token: 200 with `data.valid=false` or 401 — api-service treats non-200 as invalid.
- Default `API_AUTH_URL`: `http://auth-service:3000`.

## 4. Environment variables (compose supplies exactly these)

api-service (viper, prefix `API_`): `API_POSTGRES_DSN`, `API_REDIS_HOST`,
`API_REDIS_PORT`, `API_REDIS_PASSWORD`, `API_RABBITMQ_PASSWORD`, `API_AUTH_URL`,
`MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_USE_SSL`,
`MINIO_BUCKET_NAME`. RabbitMQ host list comes from config default or
`API_RABBITMQ_HOSTS` (add binding if missing).

auth-service (zod env schema — names FROZEN): `NODE_ENV`, `PORT`,
`DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_NAME`, `DATABASE_USER`,
`DATABASE_PASSWORD`, `JWT_SECRET` (min 32 chars), `LOG_LEVEL`, `LOG_PRETTY`,
`RATE_LIMIT_MAX_REQUESTS`, `TOKEN_VALIDATION_RATE_LIMIT_MAX` (validate is
service-to-service traffic — compose sets lab-sized values).

operational-workers: `RABBITMQ_URL` (amqp://guest:guest@rabbitmq:5672/),
plus per-worker vars they already define (keep names, document in README).

graphrag-service (pydantic-settings): `RABBITMQ_URL`, `MONGODB_URI`,
`MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `HEALTH_PORT` (default 8080).
Keep existing extra vars but document them.

## 5. Health & observability

- api-service: `GET /health` (liveness), `GET /ready` (checks PG+Redis+RabbitMQ), `GET /metrics` (Prometheus, port 8081).
- auth-service: `GET /health`, `GET /ready` (checks PG), `GET /metrics`.
- graphrag-service: `GET /health` on `HEALTH_PORT` (8080); Prometheus metrics on `METRICS_PORT` (8081).
- workers: `GET /health`, `GET /ready`, `GET /metrics` on `HEALTH_PORT` (8080); structured logs.
- RabbitMQ: `rabbitmq_prometheus` plugin on 15692, per-object queue metrics enabled.
- Prometheus scrapes all of the above per `scripts/compose/prometheus.yml`; Grafana
  (host port 3001) auto-provisions the "Lab Overview" dashboard.
- All services: JSON structured logging, graceful shutdown on SIGTERM/SIGINT (finish in-flight, close connections).

## 6. Rules for refactoring agents

1. Edit ONLY inside your assigned directory. Root files, `deployment/`,
   `documentation/`, `graph-worker/shared/` belong to the orchestrator.
2. NEVER run `git add/commit/checkout/stash` — the orchestrator commits.
3. Do not rename queues, exchanges, routing keys, HTTP paths, response shapes,
   ports, or env var names listed above. Internal code structure is yours to improve.
4. Your service must pass its own build + tests before you report done.
5. Do not run `docker build`/`docker compose` (slow); make Dockerfiles correct by inspection.
