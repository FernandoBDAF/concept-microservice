# Configuration surface

Every environment variable the service reads, its default, and its constraint.
Validated by a zod schema at startup (`src/config/env.ts`); an invalid value
exits the process with a descriptive error. In `NODE_ENV=test` (or under
Vitest), `JWT_SECRET` and `DATABASE_PASSWORD` are auto-filled so unit tests run
without a real environment.

## Required

| Variable | Default | Constraint |
|---|---|---|
| `DATABASE_PASSWORD` | — | non-empty string. **Required.** |
| `JWT_SECRET` | — | string, **min 32 chars**. **Required** (also the HS256 signing/fallback secret). |

## Server

| Variable | Default | Constraint |
|---|---|---|
| `NODE_ENV` | `development` | one of `development` \| `production` \| `test` |
| `PORT` | `3000` | number |

## Database (PostgreSQL)

| Variable | Default | Constraint |
|---|---|---|
| `DATABASE_HOST` | `postgres` | string |
| `DATABASE_PORT` | `5432` | number |
| `DATABASE_NAME` | `auth_db` | string |
| `DATABASE_USER` | `auth_user` | string |
| `DATABASE_POOL_MAX` | `20` | number (max pool connections) |
| `DATABASE_SSL` | `false` | boolean (`true`/`1` enable; parsed textually, so `false` means false) |

## JWT / RS256 / JWKS (ADR-009.1)

| Variable | Default | Constraint |
|---|---|---|
| `JWT_ACCESS_TOKEN_EXPIRY` | `15m` | duration string (`s`/`m`/`h`/`d`) |
| `JWT_REFRESH_TOKEN_EXPIRY` | `7d` | duration string |
| `JWT_PRIVATE_KEY` | _(unset)_ | base64(PEM) or raw PEM. Enables RS256 signing when set with the public key. |
| `JWT_PUBLIC_KEY` | _(unset)_ | base64(PEM) or raw PEM. Published via JWKS; verifies RS256 tokens. |
| `JWT_ALGORITHM` | _(auto)_ | `RS256` \| `HS256`. Auto-selects RS256 when both keys are present, else HS256. Explicit `HS256` always wins; `RS256` without keys warns and falls back. |

Generate a keypair with `scripts/gen-keys.sh` (or `npm run keys:init`). With no
keypair the service runs HS256 on `JWT_SECRET` — fine for local dev and CI;
RS256 is the production path so consumers verify locally against the JWKS.

## Account lockout (ADR-009.6)

| Variable | Default | Constraint |
|---|---|---|
| `ACCOUNT_LOCKOUT_ATTEMPTS` | `5` | number — failed logins before lockout |
| `ACCOUNT_LOCKOUT_DURATION_MS` | `1800000` | number — lockout window (30 min) |
| `PASSWORD_MIN_LENGTH` | `8` | number — minimum password length on create/update |

## Admin bootstrap (ADR-009.7)

| Variable | Default | Constraint |
|---|---|---|
| `SEED_ADMIN_EMAIL` | _(unset)_ | email. When both seed vars are set and the user is absent, an admin is created on startup (idempotent). |
| `SEED_ADMIN_PASSWORD` | _(unset)_ | string |

## Observability

| Variable | Default | Constraint |
|---|---|---|
| `METRICS_ENABLED` | `true` | boolean — enables `/metrics` + Prometheus default collection |
| `METRICS_PREFIX` | `auth_service_` | string — prefix for emitted metric names |
| `LOG_LEVEL` | `info` | one of `fatal`/`error`/`warn`/`info`/`debug`/`trace`/`silent` |
| `LOG_PRETTY` | `false` | boolean — human-readable logs instead of JSON |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | _(unset)_ | string. Tracing is enabled **only** when set (OTLP/HTTP). |
| `OTEL_SERVICE_NAME` | `auth-service` | string — service name on exported spans |

## Trimmed on extraction (NOT read by this template)

These lab-specific knobs were removed; document/re-add only if you reintroduce
the feature (see README "Extraction & trim"):

- `RATE_LIMIT_WINDOW_MS`, `RATE_LIMIT_MAX_REQUESTS`, `TOKEN_VALIDATION_RATE_LIMIT_MAX`
  — the in-app rate limiter (EXP-08 apparatus). Rate limiting belongs at the
  ingress (ADR-009.5); account lockout still guards credential brute-force.
- `API_SERVICE_URL` — a consumer-coupling reserved var, unused.
