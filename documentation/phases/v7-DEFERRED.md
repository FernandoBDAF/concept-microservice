# Phase v7 — deferred implementation & validation ledger

**Context:** v7 onboards two real guests (mycelium, loam). In this pass the
**lab-side onboarding surface** landed and was statically verified (kustomize
renders, compose config parses, generator regenerates, `make verify` green).
Both guest repos **are present** as siblings under the shared `forest/` root
and were **re-reconned 2026-07-20** — `mycelium` (formerly KnowledgeManager)
at `/home/fbarroso/forest/mycelium` (main @06d2651) and `loam` at
`/home/fbarroso/forest/loam` (main @29044d8). So the guest-side changes are
**no longer blocked by a missing checkout** (the earlier premise); they are
specified against verified upstream state (v7-HANDOFF.md) and land as PRs in
those repos. The one hard precondition that remains for *running* anything is a
**live cluster/AWS session** (plus the guest images, which don't exist yet).
This file is the honest ledger of what remains before `lab-v7.0` can be tagged.

## What landed this pass (lab-side, static-verified)

- `guests/mycelium/` — compose project (42xx), k8s base (ns, netpols with the
  fake-mode no-egress-to-OpenAI spend-proof, component deploys, ingress),
  obs ServiceMonitors, a Grafana dashboard, launch.yaml. Images referenced,
  not built (guest-side). **Provisional vs. the 2026-07-20 recon:** these
  manifests predate it and over-decompose the app — GraphDash was **retired
  upstream** in mycelium's phase 4 (no `:3001`, no dir), and the stages
  (`:8080`) and graph (`:8081`) APIs are now **one consolidated FastAPI/uvicorn
  process** (`GraphRAG/src/app/api/fastapi_app/main.py:266-278`), not two
  images. The component set shrinks when KM-2 lands the real images;
  reconciled there (YAML is handled elsewhere).
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

## Guest-side PRs (repos present & re-reconned — genuine greenfield/refactor)

Not blocked by a missing checkout (the earlier premise): both repos are on this
machine and were re-reconned 2026-07-20. What follows is real engineering scoped
against verified upstream state — mostly **greenfield** (KM never had fake-LLM,
containers, or a stage-level broker) plus one **refactor** (L-1). Each lands in
the guest repo, specified in `documentation/phases/v7-HANDOFF.md`.

| ID | Repo | Work (real scope, re-reconned 2026-07-20) | Blocks |
|---|---|---|---|
| KM-1 | mycelium | Fake-LLM mode — **greenfield**: no `LLM_MODE`/fake path exists today; the `dry_run` flag only skips DB writes and still spends on OpenAI/Voyage. Add deterministic fixtures for the LLM client (`src/lib/llm/client.py`) + Voyage embedder (`src/domain/ingestion/stages/embed.py`) that refuse real keys. | EXP-70 |
| KM-2 | mycelium | Containerize — **greenfield**: no Dockerfiles anywhere, CI builds no images. One **consolidated FastAPI image** (binds `:8080`+`:8081` in one process, `main.py:266-278`) + a StagesUI image; **no separate graph-api image, no GraphDash image** (retired). `/health` **already exists** (`fastapi_app/main.py:179`) and `/metrics` is served; `/ready` is the real gap. | EXP-70, guest-up |
| KM-4 | mycelium | Stage-level queue-mode on the lab RabbitMQ envelope (`km.stage.*`, ADR-007.4) + stage idempotency audit. **Note:** mycelium already has a **real arq/Redis RUN queue** (phase-3, `src/app/workers/`) enqueuing whole pipeline runs — that is *not* this; KM-4 is the separate/future migration of **stage boundaries** onto RabbitMQ. | EXP-70 |
| L-1 | loam | Callsite indirection — route the **8 files / 9 `docker()` sites** (incl. **both** calls in `orchestrate/index.ts` @2070/@2179 — the main engine) through one seam; **also** account for the host-side `noSandbox` path in `orchestrate/operator.ts:662`. sandcastle's `SandboxProvider` union has **0 uses** in loam, so the indirection is genuinely required. | L-2 |
| L-2 | loam | k8s-Jobs adapter — must **first resolve the execution-model split**: loam is **host-driven** today (host holds the handle and drives the agent via streamed `exec`; the container just runs `sleep infinity`), while the example Job is **fire-and-forget autonomous** (and there is no `loam-agent` binary in the image). Then add resource/`activeDeadlineSeconds`/`ttlSecondsAfterFinished` knobs — **none exist today** (loam has only software timeouts). | EXP-72, EXP-73 |

> **Step 0 (guest-side, security) — corrected 2026-07-20:** there is **no
> `.env` in `/home/fbarroso/forest/loam`** (verified: `stat`/`find`/`git status
> --ignored` all show absence) and **no token committed** anywhere (the only
> `sk-ant-` string is the validator regex at `config.ts:318`); `.env` is
> correctly gitignored. The old "rotate the live token before building" hard
> gate is therefore **stale for this checkout** — downgrade it to a **pre-deploy
> verification step**: confirm no `.env` with live tokens exists before building
> images (none present now) and that ignore-hygiene holds. Keep the awareness;
> loam injects creds at run time via `agentAuthEnv()` (`config.ts:313`).

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
- **mycelium metric names are now known (re-reconned 2026-07-20)** — the real
  Prometheus names are **unprefixed**: `chunks_processed` (labels `stage`/
  `stage_name`) and `stage_duration_seconds` (`src/lib/metrics/`); there is **no
  `mycelium_*` prefix**, and `/metrics` already exists. The dashboard can be
  reconciled to these now rather than waiting on KM-2.
- **Broker topology is additive but destructive to *change*:** the
  `mycelium` vhost is new (safe to add), but once mounted, altering a
  queue's args hits PRECONDITION_FAILED — `make nuke`/recreate during dev
  (v4-HANDOFF ground rules).

## This PR's lineage

The original v7 lab-side work was PR #6 (`phase/v7`), stacked on `phase/v6`
(PR #5). **PR #5 is now merged to `main`**, so this corrective pass is rebased
onto `main` and targets `main` directly — no stacking. The systems-registry
loader and controld action-API this onboarding plugs into arrived with v6 (now
on `main`).
