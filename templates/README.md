# Templates (ADR-010.3 · phase v8)

The lab's original motivation, made deliverable: hardened pieces become
**copy-ready templates** a real project bootstraps from in under a day
(PRD success metric — EXP-80 measures it honestly). Rules:

- A template ships **only patterns experiments have beaten on** — every
  claim in a template README cites the experiment that proved it.
- Copy-then-own: no semver, no shared library. A piece graduates to its
  own repo on first real adoption — procedure in
  [GRADUATION.md](GRADUATION.md).
- Each template carries: `README.md` (adapt-in-a-day guide), a bootstrap
  test (`test/bootstrap.sh` — proves a fresh copy builds and passes its
  smoke), its k8s base + compose snippet, and its own minimal CI workflow
  runnable in the consuming repo.

| Template | Source of truth (extraction origin) | Status |
|---|---|---|
| [auth-service](auth-service/) | `auth-service/` post-v4 (JWKS, sessions, rotation, lockout, roles) | cut — 32 tests green; live smoke pending (v8-DEFERRED D2) |
| [worker-go](worker-go/) | `graph-worker/operational-workers/internal/common/*` post-v4 (envelope, consume loop, retry tiers, idempotency, results) | cut — 28 tests green; live smoke pending (v8-DEFERRED D2) |
| [api-publisher](api-publisher/) | `api-service` post-v4 (outbox+relay, typed submission, JWKS middleware) | cut — 22 tests green; live smoke pending (v8-DEFERRED D2) |
| [patterns/](patterns/) | docs, not code (cache-aside, storage pipeline, deploy shapes) | written — claims code-verified against post-v4 `main` |

Extraction state (2026-07-19): v8-HANDOFF §§1–5 executed on
`feat/v8-execution`; §§6–7 (EXP-80..82 runs, tag) tracked in
[../documentation/phases/v8-DEFERRED.md](../documentation/phases/v8-DEFERRED.md).
Because v4's EXP-40..45 are authored but not yet live-run, v4-proven-by
citations in template READMEs carry "authored; live run pending" markers —
drop them only when the v4 ledger closes.

The one Helm chart (ADR-002.1) packages **worker-go**: `worker-go/chart/`
— the deliberate Helm-literacy exercise; kustomize remains the lab's tool.
