# Session handoff — the v3→v8 expedited pass (2026-07-19)

**Read this first if you are a new session picking up this repo.** It
explains the six open PRs, what is real vs skeleton in each, the working
methodology that produced them, and the traps that will bite you if you
don't know about them.

## TL;DR

Phases v3–v8 (`documentation/phases/`) were originally delivered in one
expedited pass as a **stack of six unmerged PRs**, each branch based on
the previous one — **v3 fully implemented; v4–v8 settled architecture +
compilable skeletons + step-by-step `vN-HANDOFF.md` execution plans**
written so a later session could finish them mechanically. Execution
passes have since filled in the skeletons: v3 and v4 are merged, v5–v8
are code-complete on their phase branches (status prose + PR table below
track what is merged, in PR, or pending live runs).

## The PR stack

Status now: **v3 is merged and validated, tagged `lab-v3.0`** (deferred
exit runs EXP-30..34 passed); **v4 is merged to main** (PR #3, merge
`a4c4fa1`) with the graphrag cold-start hotfix (PR #15) on top, but its
exit runs (EXP-4x) haven't run, so **no `lab-v4.0` tag yet**; **v5 is in
PR (#4)** — the AWS track, target of this fix wave. The remaining v6–v8
stack is still unmerged. Merge order is forced bottom-up (each PR's base
is the previous branch; GitHub retargets bases as you merge). A phase is
tagged only after its exit/deferred runs pass — so tags run `lab-v1.1`,
`lab-v2.0`, `lab-v3.0` and stop there.

| PR | Branch → base | State | Start here |
|---|---|---|---|
| #2 | `phase/v3-observability` → `main` | **Merged + tagged `lab-v3.0`.** Obs stack, OTel end-to-end, logs, alerts→ntfy, status page + controld, HOST_CONTRACT v0 + hello-guest. Exit experiments EXP-30..34 **run and passed** (six defects fixed) | `documentation/phases/v3-DEFERRED.md` |
| #3 | `phase/v4` → v3 | **Merged to main** (PR #3 + graphrag hotfix PR #15). Full implementation: broker topology as data (`deploy/rabbitmq/definitions.json`, generated), experiment YAML schema, idempotency, retry tiers, outbox, JWKS, loadgen, runner. Static battery green; **live-run validation (EXP-4x) + `lab-v4.0` tag still pending** | `documentation/phases/v4-HANDOFF.md` |
| #4 | `phase/v5` → v4 | **In PR (#4); code-complete + review fix wave landed.** Terraform (three stacks, `terraform validate`-clean), aws overlay, session runbook, OIDC pipeline. **Never applied on AWS** — step-0 account + EXP-50..55 pending | `documentation/phases/v5-HANDOFF.md` |
| #5 | `phase/v6` → v5 | Skeleton: `systems/` registry (usable), controld action-API contract as compiling 501 stubs | `documentation/phases/v6-HANDOFF.md` |
| #6 | `phase/v7` → v6 | Recon + plans: real recon of both guest repos, two deployment plans, DRAFT systems entries, guest-side change specs | `documentation/phases/v7-HANDOFF.md` |
| #7 | `phase/v8` → v7 | Skeleton: template scaffolds w/ extraction maps, the one Helm chart (lints), graduation runbook. Extraction gated on v4 execution | `documentation/phases/v8-HANDOFF.md` |

Verification state at authoring: on `phase/v3-observability` (and every
later branch) `make verify` is green across all seven projects (4 Go
modules, auth-service typecheck+build+23 tests, graphrag compileall, plus
the three new Go modules), `make drift-check` passes, all kustomize
renders are clean, `helm template`/`helm lint` pass for everything
helm-shaped. Nothing requiring a live cluster or AWS was executed.

## Methodology used (and recommended for continuation)

1. **Stacked branches, one PR per phase.** Phases are sequential by
   design; every phase branch contains all previous phases. Each PR's
   diff shows only its own phase.
2. **Recon before code.** Three parallel read-only recon agents mapped
   deploy conventions, Go internals, and auth/graphrag/docs before any
   edit. The per-repo conventions that recon surfaced (netpol
   default-deny style, `$(VAR)` secret idiom, drift-check SERVICE_MAP,
   named ports, `app:` labels) were treated as law in all new code.
3. **Fan-out with disjoint file ownership.** v3's five implementation
   agents each owned a disjoint file set; **shared files (Makefile,
   docker-compose.yml, prometheus.yml, CI, drift-check, init-secrets)
   were reserved for the orchestrator** and edited once, at integration
   time, from written per-agent integration notes. This avoided every
   merge conflict. Keep this rule if you fan out.
4. **Verify battery per phase, commit per phase.** `make verify` +
   `make drift-check` + kustomize/helm renders before each commit; every
   commit message states honestly what was and wasn't validated.
5. **Honest deferral ledgers.** Anything not executed is *registered*,
   not hidden: v3's unrun experiments in `v3-DEFERRED.md`; v4–v8's whole
   execution in their `vN-HANDOFF.md`s. The handoffs are written as
   ordered steps with exact file paths, code/SQL to paste, and per-step
   acceptance checks — execute them top to bottom, don't re-litigate the
   architecture (decisions cite their ADRs).
6. **Skeletons must compile.** Every stub landed green (`TODO(vN)`
   markers, not broken code), so CI stays meaningful during the long
   unmerged period.

## Traps and open flags

- ⚠️ **Agent credentials (corrected 2026-07-20):** the earlier flag of a
  live `CLAUDE_CODE_OAUTH_TOKEN`/`ANTHROPIC_API_KEY` in `loam/.env` did
  **not reproduce** — there is no `.env` in `/home/fbarroso/forest/loam`
  and no token committed anywhere (verified by `stat`/`find`/`git grep`;
  `.env` is gitignored + untracked). Treat it as a pre-deploy verification
  step, not a hard blocker (`LOAM_DEPLOYMENT_PLAN.md`).
- **Stale paths in older docs:** the guest repos live at
  `/home/fbarroso/forest/mycelium` and `/home/fbarroso/forest/loam` — older
  paths (`~/repo/mycelium`, `~/repo/Raine/loam`, `~/repo/forest/...`) are
  wrong; the v7 docs carry the correction.
- **`loam validate` hook noise:** a workspace-level hook runs on every
  file write and reports 11 pre-existing errors about
  `documentation/decisions/*.md` frontmatter. It's on `main`, it's not
  yours, the writes still succeed. Fix belongs in a dedicated pass
  (either add loam frontmatter to the ADRs or exclude the dir in the
  forest-level `loam.config.json`).
- **Broker topology is destructive to change:** once v4's
  `definitions.json` is mounted, queue-arg changes hit
  PRECONDITION_FAILED on live brokers — `make nuke`/recreate during
  development (v4-HANDOFF ground rules).
- **definitions.json declares user `guest`** (load_definitions skips
  default-user creation); on kind the password is rotated by
  init-secrets — see v4-HANDOFF §A1 before mounting.
- **Unverified pins:** obs-stack helm chart versions were
  network-verified on 2026-07-19, but container image tags (opensearch
  2.19.1, fluent-bit 3.2.10, ntfy v2.11.0) were not pull-verified; first
  `make obs-up` confirms.

## How to resume work

1. Pick the lowest unmerged phase. Check out its branch.
2. Read its `vN-HANDOFF.md` (v3: `v3-DEFERRED.md`) end to end, then the
   phase doc for the why.
3. Execute steps in order; run each step's acceptance check plus
   `make verify && make drift-check`; commit per step onto the phase
   branch (its PR updates in place).
4. When a phase's exit experiments pass: write the experiment write-ups
   in `documentation/experiments/`, update the phase doc status, and only
   then tag `lab-vN.0` (at merge time).

Project memory for assistant sessions also records this state
(`expedited-v3-v8-pass`), but this file is the canonical, in-repo record.
