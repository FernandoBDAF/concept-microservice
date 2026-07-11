# Phase v8 — Extraction & reuse

**Status:** pending · **Depends on:** v4 (hardened patterns), ideally v7
(patterns proven against real guests) · **Exit tag:** `lab-v8.0` ·
**Decisions in force:** ADR-010.3 (in-repo templates), ADR-002.1 (the one
Helm chart), ADR-009 (auth template content), ADR-008 (worker template
content)

## Mission

The original motivation, made deliverable: the lab's hardened pieces become
**copy-ready templates** a real project bootstraps from in under a day. Every
template ships only patterns that experiments have beaten on.

## Context a fresh session needs

- By now the lab embodies: RS256/JWKS auth with sessions/rotation/roles
  (ADR-009), workers with retry-backoff/idempotency/results (ADR-008),
  definitions.json topology, outbox publishing, cache-aside with known
  failure modes (EXP-12 findings), storage pipeline with status lifecycle,
  kustomize deploy shapes, scored-experiment harness, phased CI.
- Templates live in-repo first: `templates/<piece>/`; a piece graduates to
  its own repo when a real project adopts it (ADR-010.3).

## Work breakdown

1. **Template: auth-service** — trimmed auth-service (JWKS, sessions,
   rotation, lockout, roles middleware, metrics/health), config surface
   documented, seed/init scripts, its k8s base + compose snippet, and a
   bootstrap test (below). "How to adapt" README (rename, schema, claims).
2. **Template: async worker (Go)** — the operational-workers common core as
   a skeleton: envelope, consume loop w/ reconnect, retry tiers, SETNX
   idempotency guard, results publishing, metrics/health server; one example
   processor; definitions.json fragment generator for a new queue.
3. **Template: API publisher** — outbox table + relay, typed task submission,
   JWKS verification middleware (Go), the CONTRACTS.md skeleton.
4. **Pattern docs (not code):** cache-aside (with the EXP-12-discovered
   failure behavior and stampede caveats), storage pipeline (presigned URLs,
   orphan reconciliation sweep), deploy shapes (kustomize layout, probes/
   limits/PDB defaults, netpol starter).
5. **The one Helm chart (ADR-002.1):** package exactly one template (worker
   or auth) as a proper chart — values design, helpers, README — as the
   deliberate Helm-literacy exercise; document kustomize-vs-helm impressions
   in the write-up.
6. **CI for templates:** each template carries its own minimal workflow
   (build+test) runnable in the consuming repo; the lab's CI additionally
   runs template bootstrap tests on changes under templates/.
7. **Graduation runbook:** documented procedure for promoting a template to
   its own repo on first real adoption (history subtree split, versioning,
   feedback flow back to the lab).

## Out of scope

Publishing/marketing templates publicly, semver-maintained library releases
(templates are copy-then-own), supporting stacks the lab doesn't use.

## Exit experiments

- **EXP-80 — Bootstrap under a day:** from an empty repo, assemble a small
  real-ish service (auth template + one worker + API publisher) following
  only the templates' READMEs; `make verify`-equivalent green and a smoke
  experiment passes; wall-clock logged (target < 1 day, honest count).
- **EXP-81 — Template regression:** the templates' own CI passes; a
  deliberately-broken copy fails its bootstrap test (the harness catches
  real breakage).
- **EXP-82 — Helm exercise:** the chart installs the piece on kind with
  overridden values; uninstall clean; write-up records the kustomize/helm
  comparison honestly.

## Acceptance

- [ ] EXP-80..82 pass with write-ups (EXP-80's timing is the headline metric)
- [ ] templates/ complete with per-piece READMEs + bootstrap tests
- [ ] Graduation runbook exists
- [ ] PRD success metric "real project bootstrapped < 1 day" demonstrably met
- [ ] Tag `lab-v8.0`
