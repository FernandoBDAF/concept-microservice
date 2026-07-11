# Phase v1 — Foundation (SHIPPED 2026-07)

**Status:** ✅ shipped (merged `15825f9`) · **Validated by:** live E2E during
the refactor + EXP-01..09 · **Record, not a brief.**

## What this phase produced

The consolidated architecture, made real and verified:

- **Services:** api-service (Go/Gin: profiles, documents, tasks; direct
  Postgres/Redis/RabbitMQ/MinIO), auth-service (TS/Express: register, login,
  refresh, validate), three Go operational workers (email/image/profile),
  graphrag-service (Python consumer, stub mode).
- **Orchestration:** root `docker-compose.yml` (13 containers: infra +
  migrations + bucket init + services + monitoring), root `Makefile`
  (up/infra/down/nuke/logs/verify + sims).
- **Contracts:** `CONTRACTS.md` (topology/ports/env/health),
  `graph-worker/shared/contracts/` (envelope, routing keys) — reconciled with
  code after the profile-tasks drift was found dead.
- **Monitoring:** Prometheus scraping every service + RabbitMQ per-queue
  metrics; Grafana with provisioned Lab Overview dashboard (8 panels).
- **Simulation harness:** k6 HTTP load (smoke/load/burst), queue publisher
  (flood/poison via mgmt API), worker-outage drill, scale target.

## Defects fixed on the way (highlights)

Envelope double-nesting (workers rejected 100% of messages); missing DLQ
topology; dead profile.task pipeline (exchange drift); RabbitMQ shutdown
panic; breaker tripping on 401 bursts; metrics not actually on :8081; zod
boolean-env footgun; non-idempotent migrations; hardcoded validate rate limit
(the live incident — see CONCEPTUAL_REVIEW §2).

## Verification record

`make verify` green across all four projects; all 7 images build (native
arch); full-stack boot; live E2E: register → login → authenticated CRUD →
email+profile tasks consumed; 50-VU burst 99.96% checks at ~179 req/s; poison
lands in DLQs exactly as designed.

## Key references

Root README · CONTRACTS.md · documentation/refactor/2026-07-full-refactor.md ·
documentation/review/CONCEPTUAL_REVIEW.md (the findings v4 fixes).
