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

## Verification gates (phase 5)

- [ ] `cd api-service && go build ./... && go vet ./... && go test ./...`
- [ ] `cd graph-worker/operational-workers && go build ./... && go test ./...`
- [ ] `cd auth-service && npm run typecheck && npm run build && npm test`
- [ ] `python3 -m compileall graph-worker/graphrag-service/src`
- [ ] `docker compose config` valid; every env var in CONTRACTS §4 supplied
- [ ] Routing keys/exchanges/queues identical in publisher (Go), workers (Go), graphrag (Py)
- [ ] Auth validate request/response shapes match between Go client and TS handler

## Known leftovers for your review

Filled in during the run — see FINAL_REPORT section of the run summary and
`git log` messages for anything deferred.
