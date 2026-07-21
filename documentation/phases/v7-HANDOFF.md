# Phase v7 handoff — guest systems (mycelium & loam)

**Audience:** the session(s) that finish v7. Scope decision in force:
this branch is **lab-side only** — guest-side changes are specified here
and land as PRs in the guest repos. Both repos actually live at
`~/repo/forest/mycelium` and `~/repo/forest/loam` (phase-doc paths are
stale). Recon-based deployment plans (read them first — they carry the
architecture): `documentation/deployment/KM_DEPLOYMENT_PLAN.md`,
`documentation/deployment/LOAM_DEPLOYMENT_PLAN.md`.

> ⚠️ Step 0, before anything: rotate the live agent credentials sitting in
> `loam/.env` (see the loam plan's security flag).

Inherited on `phase/v7`: draft systems entries (`systems/mycelium.yaml`,
`systems/loam.yaml` — marked DRAFT) and both deployment plans.

> **UPDATE 2026-07-19 — lab-side onboarding landed.** The steps that live
> in *this* repo are done and statically verified: **KM-3** (`guests/mycelium/`
> + the `mycelium` broker vhost in `generate-definitions.py`), **L-3**
> (`guests/loam/`), the EXP-70..74 scored defs, and the systems-entry notes.
> What remains is the **guest-side** work (KM-1/2/4, L-1/2 — repos absent
> this pass) and every experiment *run* (no live cluster/AWS). The honest
> ledger of what's left, with entry points, is
> [v7-DEFERRED.md](v7-DEFERRED.md). The KM/loam sections below are the
> guest-side spec — still the source of truth for those PRs.

## KM track

**KM-1 (mycelium repo) — fake-LLM mode (ADR-007.3, contract gate):**
Central seam is `src/lib/llm/client.py` (+ `src/infrastructure/llm/
openai.py` singleton) and the Voyage embedder (`stages/embed.py`).
Spec: `LLM_MODE=fake` env → clients return deterministic fixtures keyed by
`sha256(model + prompt)[:16]` from `fixtures/llm/` (committed, generated
once with real keys via a `record` mode: `LLM_MODE=record` writes through
while capturing); embedder returns `sha256`-seeded pseudo-vectors (dim
1024) so downstream similarity is stable. Fake mode must hard-fail if
`OPENAI_API_KEY`/`VOYAGE_API_KEY` are SET (proves zero-spend by
construction — EXP-70's "no key mounted" assertion inverts to "fake mode
refuses keys"). Config flag surfaced in `.env` + README.

**KM-2 (mycelium repo) — containerize:** one GraphRAG image (entrypoints:
stages-api, graph-api, pipeline CLI), two UI images. Multi-stage, non-root
10001, /health + /ready (add trivial handlers to the stdlib servers;
stages-api already has /metrics).

**KM-3 (lab repo) — onboarding:** `guests/mycelium/` per HOST_CONTRACT
(compose project, ports 4210/4211/4220/4221; k8s namespace `mycelium`,
netpols default-deny + egress mongo.lab-infra:27017 + rabbitmq:5672 +
lab-obs scrape ingress; NO egress to api.openai.com in fake mode — that's
the netpol-level spend proof). Topology: extend
`scripts/rabbitmq/generate-definitions.py` with a `mycelium` vhost
(stage queues `km.stage.<name>` + retry tiers + DLQ, same generator
pattern; ADR-007.4) — definitions stay single-sourced. ServiceMonitor +
a KM Grafana dashboard (stage counters/durations from its existing
`/metrics`). launch.yaml; un-DRAFT `systems/mycelium.yaml`.

**KM-4 (mycelium repo) — queue-mode:** `PipelineRunner --queue-mode` per
the deployment plan §2 (stage boundaries → lab envelopes on the vhost;
in-process default untouched upstream). Audit stage idempotency first
(plan's open question — upsert-shaped writes are the precondition for
retry tiers).

**Exit:** EXP-70 (fixture flood in fake mode, stages drain on its
dashboard, retry/DLQ per conventions, zero spend proven) and EXP-71 (one
budgeted real-key run; record $ + fake-vs-real stage timings).

## loam track

**L-1 (loam repo) — callsite indirection:** `createRunnerSandbox()` in
`packages/workflows/src/shared/agent.ts`; route the 7 `docker()` callsites
through it; `loam.config.json` gains `runner: docker|k8s`. Pure refactor,
its own PR, `--dry-run` output unchanged.

**L-2 (loam repo) — k8s-Jobs provider (ADR-007.5):** implement the
`IsolatedSandboxHandle` contract per the deployment plan's adapter spec —
in-pod clone via init container, agent works in-pod, branch pushed to
origin on completion, host reads `branchAheadShas` against
`origin/<branch>`; Job spec knobs (resources from `repos.<area>.k8s.*`,
`activeDeadlineSeconds` from `runTimeoutMs`, `ttlSecondsAfterFinished`,
`backoffLimit: 0`); token via `secretKeyRef` (ADR-007.6); dry-run renders
the manifest; preflight gains a `kubectl auth can-i create jobs` probe.

**L-3 (lab repo) — onboarding:** `guests/loam/` (UI 4310 + control API
4320 as one deployment, host=0.0.0.0 in-container; namespace `loam`;
ResourceQuota/LimitRange for 3 concurrent agent Jobs; Secret
`loam-agent-token`; netpols with the documented egress holes to
api.anthropic.com:443 + github.com:443). `/api/capabilities` is the
readiness probe. launch.yaml; un-DRAFT `systems/loam.yaml`.

**Exit:** EXP-72 (real workflow as a Job within wall-clock budget; logs +
branch artifact retrievable; hung agent killed by activeDeadline and
reported cleanly) and EXP-73 (OOM at limit leaves neighbors untouched —
quota proof; token absent → fast, clear failure; rotation drill).

## Joint exit

EXP-74: one guest (owner's pick) runs a scaled-down drill inside an AWS
session; cost recorded; teardown clean. Then: HOST_CONTRACT.md v0 → v1
with lessons (real-guest friction goes in the contract, not around it);
both systems entries live in Mission Control; write-ups with cost lines
for EXP-71/74; phase doc status update; tag `lab-v7.0`.

## Cross-repo mechanics

Guest-side work = branches/PRs in the guest repos (KM-1/2/4, L-1/2);
lab-side = this repo (KM-3, L-3, EXP defs, contract v1). Keep the pairing
honest: a lab-side PR that depends on an unmerged guest-side PR says so in
its description.
