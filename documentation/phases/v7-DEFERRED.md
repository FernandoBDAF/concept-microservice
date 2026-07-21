# Phase v7 — deferred implementation & validation ledger

**Context:** v7 onboards two real guests (mycelium, loam). In this pass the
**lab-side onboarding surface** landed and was statically verified (kustomize
renders, compose config parses, generator regenerates, `make verify` green),
but the two hard preconditions for *running* anything — the **guest repos**
and a **live cluster/AWS session** — were not available. Both guest repos
(`~/repo/forest/mycelium`, `~/repo/forest/loam`) are **not checked out on
this machine**, so every guest-side change is specified (v7-HANDOFF.md) and
deferred to a PR in that repo. This file is the honest ledger of what remains
before `lab-v7.0` can be tagged.

## What landed this pass (lab-side, static-verified)

- `guests/mycelium/` — compose project (42xx), k8s base (ns, netpols with the
  fake-mode no-egress-to-OpenAI spend-proof, four component deploys, ingress),
  obs ServiceMonitors, a Grafana dashboard, launch.yaml. Images referenced,
  not built (guest-side).
- `guests/loam/` — compose project (43xx), k8s base (ns `loam`, UI+control
  deploy, ResourceQuota/LimitRange for 3 concurrent agent Jobs, netpols with
  the documented egress holes to anthropic/github, placeholder
  `loam-agent-token` Secret, example agent-Job manifest), launch.yaml.
- `scripts/rabbitmq/generate-definitions.py` — **`mycelium` vhost** added
  (stage queues `km.stage.<name>` + 5s/30s/2m retry ladder + per-stage DLQ +
  `mycelium-results` loop). Lab-core (`/`) output is byte-identical (verified
  non-destructive). `deploy/rabbitmq/definitions.json` +
  `ROUTING_KEYS.generated.md` regenerated.
- `experiments/exp-70.yaml`..`exp-74.yaml` — scored exit definitions
  authored to schema v0. They **register** the exit criteria; none are
  runnable yet (see below).
- `systems/mycelium.yaml`, `systems/loam.yaml` — notes updated: lab-side
  scaffolding present, guest images pending.

## Guest-side PRs (blocked — repos absent this pass)

Each lands in the guest repo, specified in `documentation/phases/v7-HANDOFF.md`.

| ID | Repo | Work | Blocks |
|---|---|---|---|
| KM-1 | mycelium | Fake-LLM mode (deterministic fixtures; refuses real keys) | EXP-70 |
| KM-2 | mycelium | Containerize (one GraphRAG image + two UI images; /health,/ready) | EXP-70, guest-up |
| KM-4 | mycelium | `PipelineRunner --queue-mode` on the lab envelope + stage idempotency audit | EXP-70 |
| L-1 | loam | `createRunnerSandbox()` callsite indirection (pure refactor) | L-2 |
| L-2 | loam | k8s-Jobs runner adapter (in-pod clone, branch push, Job knobs, secretKeyRef, dry-run manifest, preflight probe) | EXP-72, EXP-73 |

> ⚠️ **Step 0 (guest-side, security):** rotate the live
> `CLAUDE_CODE_OAUTH_TOKEN` / `ANTHROPIC_API_KEY` in `loam/.env` before any
> image build or agent run (LOAM_DEPLOYMENT_PLAN security flag). Re-flag to
> the owner if untouched.

## Must run before tagging lab-v7.0 (needs images + live cluster/AWS)

| Item | Needs | Entry point |
|---|---|---|
| EXP-70 KM burst (fake mode) | KM-1/2/4 merged; `make guest-up G=mycelium` on kind; obs stack | experiments/exp-70.yaml |
| EXP-71 KM budgeted real run | real OPENAI/VOYAGE keys; hard budget; write-up w/ $ + timings | experiments/exp-71.yaml |
| EXP-72 agent-run lifecycle | L-1/2 merged; loam on kind; a real workflow as a Job | experiments/exp-72.yaml |
| EXP-73 agent resource envelope | L-2; OOM/quota drill; token-absent + rotation drill | experiments/exp-73.yaml |
| EXP-74 guest on AWS | **v5 (AWS track) executed**; owner picks a guest; cost recorded | experiments/exp-74.yaml |
| HOST_CONTRACT v0 → v1 | lessons from the real-guest runs above (friction goes in the contract) | documentation/HOST_CONTRACT.md |
| systems entries un-DRAFT → live | guest images build + launch verified from Mission Control (v6) | systems/{mycelium,loam}.yaml |

## Known caveats shipped knowingly

- **Guest images are referenced, not built.** `guests/*/docker-compose.yml`
  and the k8s Deployments name `localhost:5001/<component>:dev` images that
  only exist after the guest-side containerization PRs. `make guest-up`
  will fail to pull until then — by design; the manifests are the contract.
- **loam exposes no `/metrics` today** (recon). Its obs/ ServiceMonitor is a
  commented placeholder; the netpol keeps the obs-scrape ingress pattern for
  when metrics land.
- **loam agent-Job egress is broad** (`0.0.0.0/0` TCP 443, scoped to the
  agent pod label): NetworkPolicy can't match FQDNs, so anthropic/github are
  reached via a documented wide hole. FQDN-scoped egress (e.g. an egress
  gateway) is a follow-up; the UI pod stays fully locked down.
- **mycelium metric names in the dashboard are provisional** — confirmed
  against the real `/metrics` when KM-2 lands.
- **Broker topology is additive but destructive to *change*:** the
  `mycelium` vhost is new (safe to add), but once mounted, altering a
  queue's args hits PRECONDITION_FAILED — `make nuke`/recreate during dev
  (v4-HANDOFF ground rules).

## This PR's dependency

Base branch is `phase/v7`, which stacks on `phase/v6` (PR #5, unmerged). The
systems-registry loader and controld action-API this onboarding plugs into
arrive with v6. Merge order stays bottom-up.
