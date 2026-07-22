# Template: auth-service

A copy-ready authentication service (Node.js / TypeScript / Express +
PostgreSQL). RS256 signing with a published **JWKS**, refresh-token **sessions
with one-time rotation + reuse detection**, **account lockout**, and **role**
middleware. Copy it, rename it, run migrations, generate keys — adapt in a day.

> **Copy-then-own** (ADR-010.3): no semver, no shared library. Take the code and
> make it yours. Graduation to its own repo on first real adoption:
> [`../GRADUATION.md`](../GRADUATION.md).

## What you get

- **RS256 + JWKS** (ADR-009.1): tokens are signed with a private key carrying a
  `kid`; `GET /.well-known/jwks.json` publishes the public key so any consumer
  verifies **locally** — no per-request call back to auth. HS256 on `JWT_SECRET`
  is the keyless fallback for local dev / CI. Rotation-ready: `publicKeys` is an
  array; the newest signs, all published verify.
- **Sessions with rotation + reuse detection** (ADR-009.2): every refresh mints
  a new token pair and records the presented `jti` as the chain's *previous*
  token. Replaying a rotated-out token is a theft signal — the session is
  revoked and an audit row is written, and the replay is rejected `401`.
- **Account lockout** (ADR-009.6): N failed logins lock the account for a window
  (both configurable). This, not a rate limiter, is the credential-brute-force
  guard kept in the template.
- **Role middleware**: `requiresAuth(["admin"])` gates routes; `user` vs `admin`.
- **Operational surface**: `/health`, `/ready`, `/live`, Prometheus `/metrics`,
  structured pino logs, zod-validated request bodies, optional OpenTelemetry
  tracing (off unless `OTEL_EXPORTER_OTLP_ENDPOINT` is set), env-driven admin
  seed, idempotent SQL migrations applied on boot.

## Proven by (honest provenance)

Every pattern here is carried from the lab's `auth-service` and cites the
experiment that exercised it. Citations are honest about what has actually had a
live run versus what is authored and pending one (v4 deferral ledger):

- **EXP-02** — the token-validation **contract** shape (`/v1/auth/token/validate`
  response) — real history.
- **EXP-08** — the introspection **SPOF**: validating every request by calling
  auth makes auth a single point of failure. Real history; it motivated JWKS.
- **EXP-43** — retiring introspection in favour of JWKS-local verification
  (the template defaults to JWKS-only; introspection is an opt-in on the
  consumer side). **Authored; live run pending.**
- **Session rotation / reuse tests (v4 A7)** — the rotate-then-replay theft
  detection. Covered by unit tests here (`AuthService.test.ts`) and the
  bootstrap smoke. **Authored; live run pending** at the lab-integration level.

## How to adapt

**1. Copy, rename, migrate, generate keys.**
   Copy this directory into your repo. Rename the service and database
   (`DATABASE_NAME` / `DATABASE_USER`, the `name` in `package.json`, k8s/compose
   labels). Bring up Postgres and let the service apply migrations on boot (it
   runs `migrations/*.sql` idempotently), or apply them with `psql`. Mint an
   RS256 keypair:

   ```bash
   npm install
   npm run keys:init            # prints export JWT_PRIVATE_KEY=… / JWT_PUBLIC_KEY=…
   # or: scripts/gen-keys.sh --k8s <namespace> | kubectl apply -f -
   ```

   Without keys the service runs HS256 on `JWT_SECRET` (min 32 chars) — fine to
   start, switch on RS256 before anything verifies your tokens elsewhere.

**2. Edit claims in ONE place.**
   Token claims are built in `TokenService.generateTokens` → `basePayload`
   (`userId`, `email`, `role`, `jti`). Add or rename claims there; that single
   object flows into both the access and refresh tokens. **Document any claim
   your API consumers verify** — it becomes part of your contract (see EXP-02).

**3. Prove the copy with the bootstrap smoke.**
   ```bash
   test/bootstrap.sh
   ```
   Brings up a throwaway Postgres + this service and runs
   `register → login → validate → refresh-rotate → reuse rejected → logout`
   (plus a JWKS check). Green = your copy is adapted and the pattern holds.
   Requires Docker + curl; openssl optional (present → exercises RS256/JWKS).

## Configuration

Full env surface — name, default, constraint — in [`CONFIG.md`](CONFIG.md).
The essentials: `DATABASE_PASSWORD` and `JWT_SECRET` (≥32 chars) are required;
`JWT_PRIVATE_KEY`/`JWT_PUBLIC_KEY` turn on RS256; lockout and token lifetimes are
tunable.

## API

| Endpoint | Method | Auth | Purpose |
|---|---|---|---|
| `/.well-known/jwks.json` | GET | none | Public JWKS — consumers verify RS256 locally |
| `/v1/auth/login` | POST | none | Returns access + refresh tokens; opens a session |
| `/v1/auth/token/validate` | POST | none | Validates an access token (EXP-02 contract) |
| `/v1/auth/token/refresh` | POST | none | One-time rotation; reuse → `401` + session revoke |
| `/v1/auth/logout` | POST | bearer | Revokes the presented token's session |
| `/v1/users` | POST | none | Self-registration |
| `/v1/users/me` | GET | any user | Current profile |
| `/v1/users`, `/v1/users/:id`, `/v1/users/:id/role`, … | GET/PUT/DELETE/PATCH | admin | User management |
| `/health`, `/ready`, `/live`, `/metrics` | GET | none | Probes + Prometheus metrics |

## Deploying

- **Kubernetes** — [`deploy/k8s/base/`](deploy/k8s/base/): Service + Deployment +
  ConfigMap, named `http` port, `app:` labels, startup/readiness/liveness probes,
  hardened `securityContext`, resource requests/limits. Renders with
  `kubectl kustomize deploy/k8s/base` (or `kustomize build`). Create the
  `auth-service-secrets` Secret (see `secret.example.yaml`) and, for RS256, the
  `auth-service-keys` Secret (`scripts/gen-keys.sh --k8s`). Run `migrations/` as
  a one-shot Job (or rely on the boot-time runner).
- **Compose** — [`compose.snippet.yml`](compose.snippet.yml): service + postgres
  fragment to drop into a consuming repo's `docker-compose.yml`.

Rate limiting is **not** in the app: it belongs at the ingress (ADR-009.5;
per-IP nginx annotations on the auth host). Account lockout covers credential
brute-force at the app layer.

## Extraction & trim (what changed vs the lab source)

Extracted from the lab's post-v4 `auth-service/` (the JWKS/sessions/rotation/
lockout/roles core). Trimmed as lab-specific surface:

- **In-app rate limiter + its envs** (`RATE_LIMIT_*`, `TOKEN_VALIDATION_RATE_LIMIT_MAX`)
  — the EXP-08 apparatus. Moved to the ingress per ADR-009.5.
- **OpenAPI docs generation** (`swagger-ui-express`, `zod-to-openapi`, the
  `/api-docs` route) — the zod **validation** schemas (`src/schemas/`) are kept;
  only doc *generation* was dropped.
- **Audit-log niceties** — the audit trail records exactly one event,
  `REFRESH_TOKEN_REUSE` (the theft signal); per-request IP/user-agent capture
  and broader trails were removed.
- **Consumer coupling** — `API_SERVICE_URL` and the introspection default (the
  template defaults to JWKS-only).
- **Migrations consolidated** — the lab's historical add-then-drop `salt` column
  (ADR-009.6) is collapsed into a clean baseline (bcrypt salts internally).

Kept deliberately, though not in the skeleton's trim list: the OpenTelemetry
tracing hooks (gated off unless `OTEL_EXPORTER_OTLP_ENDPOINT` is set) — a legit,
zero-cost-when-unused observability pattern.

## Development

```bash
npm install
npm run dev        # tsx watch (needs reachable Postgres + DATABASE_PASSWORD + JWT_SECRET)
npm run typecheck
npm run build      # ESM to dist/
npm test           # unit + route tests (no DB needed — DB is mocked)
```

CI: [`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs install →
typecheck → build → test, runnable as-is in a consuming repo.
