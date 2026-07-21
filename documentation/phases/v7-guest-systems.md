# Phase v7 — Guest systems: mycelium (formerly KnowledgeManager) & loam

**Status:** recon + plans + handoff (2026-07-19), then **lab-side
onboarding landed** (2026-07-19: `guests/mycelium/` + `guests/loam/`
manifests, the `mycelium` broker vhost, EXP-70..74 scored defs — all
statically verified). Guest-side code (KM-1/2/4, L-1/2) and every
experiment *run* remain deferred: repos absent + no live cluster/AWS this
pass. Ledger: [v7-DEFERRED.md](v7-DEFERRED.md); execute guest-side via
[v7-HANDOFF.md](v7-HANDOFF.md). Both repos live under `~/repo/forest/`,
not the paths below · **Depends on:** v6 (systems model), v5
(AWS sessions) ·
**Exit tag:** `lab-v7.0` · **Decisions in force:** ADR-007 (all), ADR-005.5,
ADR-008 (conventions KM adopts), ADR-009.3 (secrets)

## Mission

The two real systems onboard as guests, each with a **written real-deployment
plan** (the project's original point: plan how they deploy for real work, and
rehearse here first). mycelium's pipeline runs on the lab's queue
conventions; loam's agent runs execute as Kubernetes Jobs. Both launchable
from Mission Control, observable in the shared stack, drilled by their own
experiments.

## Context a fresh session needs

- **HOST_CONTRACT.md** (v3) proven by hello-guest; systems registry (v6).
  Port blocks: 42xx = KM, 43xx = loam (ADR-007.2).
- **mycelium** (`~/repo/mycelium`): GraphRAG ingestion
  pipeline (YouTube → knowledge graphs), GraphDash + StagesUI front-ends,
  systemic-control module; rich observability analyses exist in its docs.
  Decisions: pipeline onboards first with a **mandatory deterministic
  fake-LLM mode** (ADR-007.3); it **adopts the lab's envelope/topology/retry
  conventions** — the migration is the exercise (ADR-007.4). Shared infra by
  default: own Postgres DB / Mongo database / MinIO-S3 bucket / RabbitMQ
  vhost (ADR-007.1).
- **loam** (`~/repo/Raine/loam`): TS monorepo; workflows launch Claude Code
  agents in per-run Docker sandboxes (.env holds CLAUDE_CODE_OAUTH_TOKEN);
  execution via sandcastle; local read-only knowledge UI. Decisions: agent
  runs become **k8s Jobs** via a loam-side runner adapter (ADR-007.5);
  secrets via k8s Secrets → Secrets Manager (ADR-007.6); experiments assert
  the operational envelope only (ADR-007.7).
- Both are living repos under active development — onboarding work happens
  against *their* codebases too; plan cross-repo changes as PRs there, lab
  config here.

## Work breakdown

1. **KM recon + plan:** map the actual pipeline stages/data flows; write
   `documentation/deployment/KM_DEPLOYMENT_PLAN.md` — production target
   shape (compute for pipeline stages, stores, cost model incl. LLM spend,
   scaling knobs) + what the lab rehearsal covers.
2. **KM fake mode (KM-side):** deterministic LLM stub (fixtures keyed by
   input hash) + config flag; contract requirement before any lab drill.
3. **KM onboarding:** containerize pipeline (+ UIs second); stage messages
   ported to the lab envelope; topology (its exchanges/queues/retry/DLQ)
   added to definitions.json under its vhost; ServiceMonitors + a KM Grafana
   dashboard; systems-registry entry; launchable via Mission Control.
4. **loam runner adapter (loam-side):** implement the Jobs backend for
   sandcastle/loam workflows — Job spec (image, resource limits, TTL,
   activeDeadlineSeconds), token injection from Secrets, log capture to the
   lab's pipeline, artifact (branch/diff) retrieval; `--dry-run` parity.
5. **loam onboarding:** systems-registry entry (its knowledge UI as the
   service, agent runs as Jobs in namespace `loam`); quotas
   (ResourceQuota/LimitRange) sized for N concurrent agents; write
   `documentation/deployment/LOAM_DEPLOYMENT_PLAN.md` — the "agent farm"
   production shape (queue of runs → Jobs on EKS → artifacts/logs →
   knowledge updates).
6. **Guest experiments** (below) + at least one AWS session including a
   guest (EXP-74).

## Out of scope

KM real-key bulk ingestion (single budgeted run only), loam multi-tenant
anything, deep loam-UI/Mission-Control integration (ADR-005.5), rewriting
either project's internals beyond the onboarding surface.

## Exit experiments

- **EXP-70 — KM pipeline burst (fake mode):** N videos' worth of fixture
  ingestion flood; stages drain per its dashboard; retry/DLQ behavior per
  lab conventions; zero LLM spend proven (no key mounted).
- **EXP-71 — KM budgeted real run:** one small real-key ingestion with a
  hard budget; compare fake-vs-real stage timings; record spend in write-up.
- **EXP-72 — Agent-run lifecycle:** launch a real loam workflow as a Job:
  completes within wall-clock budget, logs+artifacts retrievable, sandbox
  cleaned (ADR-007.7). Then the hung-agent drill: an agent forced to stall
  hits activeDeadline and is killed+reported cleanly.
- **EXP-73 — Agent resource envelope:** an agent run driven into memory
  pressure OOMs at its limit without disturbing neighbors (quota proof);
  token absent → run fails fast with a clear secret error (rotation drill:
  rotate the Secret, next run picks it up).
- **EXP-74 — Guest on AWS:** one guest (owner's pick) runs a scaled-down
  drill inside an AWS session; cost recorded; teardown leaves nothing.

## Acceptance

- [ ] EXP-70..74 pass, written up (EXP-71/74 include cost lines)
- [ ] Both deployment plans written and rehearsed at least partially
- [ ] Both guests launchable from Mission Control; contract updated from
      draft to v1 with lessons
- [ ] Tag `lab-v7.0`
