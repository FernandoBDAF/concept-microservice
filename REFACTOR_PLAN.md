# Full Refactor & Sync Plan — worktree `full-refactor-sync`

**Date:** 2026-07-07 · **Executed autonomously** (pre-approved, ~1h budget)

## Why

The repo's consolidated-architecture restructure (api-service, auth-service,
graph-worker, deployment docs) existed only as ~1500 uncommitted files on `main`,
with no root orchestration, stale dependencies, and unverified builds. Goal:
make every active project build, test, and run together again — refactored in
parallel, synchronized through pinned contracts.

## Branch layout (how to review & sync back)

```
1efec36  main (old k8s/services architecture)
b423745  baseline: exact snapshot of your uncommitted working tree
   ...   refactor commits, one per area  ← REVIEW THESE
```

- **Review my changes only:** `git diff b423745..worktree-full-refactor-sync`
- **Adopt the result:** `git checkout main && git reset --hard worktree-full-refactor-sync`
  (your previous state stays recoverable at `b423745`; `legacy_project/` is
  gitignored and untouched on disk)
- Or cherry-pick per-area commits selectively.

## Scope & ownership (parallel agents, strict directory ownership)

| Area | Owner | Mission |
|---|---|---|
| `api-service/` (Go/Gin) | agent A | build+vet+tests green, deps current (incl. go-redis v9), contract-compliant publisher/auth/middleware |
| `auth-service/` (TS/Express) | agent B | typecheck+lint+build+tests green, deps current (jsonwebtoken v9), frozen validate contract |
| `graph-worker/operational-workers/` (Go) | agent C | 3 workers build+tests green, queue contract alignment, resilient consume loop |
| `graph-worker/graphrag-service/` (Python) | agent D | imports/deps sane (pydantic v2, aio-pika), contract-aligned consumer, health endpoint |
| Root, `deployment/`, `documentation/`, `graph-worker/shared/` | orchestrator | root docker-compose for the full stack, .env.example, Makefile, README rewrite, contract docs |
| `legacy_project/` | nobody | archived reference; gitignored |

## Pinned integration surface

See **CONTRACTS.md** — RabbitMQ routing keys/exchanges/queues + message
envelope, auth token-validate HTTP contract, env var names, ports, health
endpoints. Agents may not change these; everything internal is fair game.

## Phases

1. Baseline snapshot commit (done — `b423745`)
2. Pin contracts (`CONTRACTS.md`) — done
3. Parallel service refactors (agents A–D)
4. Root orchestration: `docker-compose.yml` (postgres+redis+rabbitmq+mongodb+minio+all services), `.env.example`, root `Makefile`, README rewrite
5. Integration verification: `go build/test` both Go modules, `npm run typecheck/build/test`, Python compile check, compose config validation, cross-service contract audit
6. Per-area commits + final report

## Verification gates (phase 5) — ALL PASSED 2026-07-07

- [x] `cd api-service && go build ./... && go vet ./... && go test ./...` (10 tests)
- [x] `cd graph-worker/operational-workers && go build ./... && go test ./...` (incl. `-race`)
- [x] `cd auth-service && npm run typecheck && npm run build && npm test` (23/23)
- [x] `python3 -m compileall graph-worker/graphrag-service/src` + core-deps venv import/wiring check
- [x] `docker compose config` valid; every env var in CONTRACTS §4 supplied
- [x] Routing keys/exchanges/queues/TTLs/retries byte-identical in publisher (Go),
      workers (Go), graphrag (Py) — audited value-by-value after reconciliation
- [x] Auth validate request/response shapes match between Go client and TS handler
      (verified byte-for-byte, incl. 401 invalid shape)

### Cross-service drift found & resolved

`profile.task` was published to legacy exchange **`tasks-exchange`** (TTL 24h)
while all docs said `profile-tasks` (TTL 1h) — the profile pipeline was dead
end-to-end. Reconciled to canonical `profile-tasks` / 1h / DLQ 24h / 3 retries
on BOTH publisher and consumer; unknown-key fallback renamed to `default-tasks`.

## Known leftovers for your review

1. **No live end-to-end run**: `docker compose config` validates and every
   project builds/tests locally, but `make up` (image builds + full stack boot)
   was not executed within the time budget. Expect the first boot to be the
   real test of the Dockerfiles.
2. **k8s manifests are stale relative to compose**: they still omit the new
   8081 metrics port (api-service) and use per-component RabbitMQ env vars
   (workers fall back correctly, but `RABBITMQ_URL` is preferred now).
3. **~2.2GB unreachable git objects**: the very first baseline commit briefly
   included `legacy_project/` before being amended away; `git gc` (from the
   main checkout, when no other sessions are active) reclaims the space.
4. auth-service `npm audit`: 16 transitive findings remain, all dev/build-time,
   fixable only via major bumps that were out of scope.
5. graphrag-service's real LLM/ML pipeline is import-guarded but functionally
   unverified (needs `requirements-graphrag.txt` + API keys); it runs in stub
   mode otherwise. Envelope-invalid messages are ACK-dropped by design, only
   processing failures go to the DLQ — revisit if you want them inspectable.
6. image-worker's simulated long task (~31s) can outlive the 10s HTTP shutdown
   window; use `terminationGracePeriodSeconds ≥ 35` in k8s (message delivery
   is still safe either way).
