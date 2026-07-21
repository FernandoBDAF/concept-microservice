# Phase v7 handoff — guest systems (mycelium & loam)

**Audience:** the session(s) that finish v7. Scope decision in force:
this branch is **lab-side only** — guest-side changes are specified here
and land as PRs in the guest repos. Both repos are **present** as siblings
under the shared `forest/` root and were re-reconned 2026-07-20:
`/home/fbarroso/forest/mycelium` (main @06d2651) and
`/home/fbarroso/forest/loam` (main @29044d8). (Earlier `~/repo/...` paths are
stale — corrected throughout.) Recon-based deployment plans (read them first —
they carry the architecture): `documentation/deployment/KM_DEPLOYMENT_PLAN.md`,
`documentation/deployment/LOAM_DEPLOYMENT_PLAN.md`.

> **Step 0 (corrected 2026-07-20):** there is **no `.env`** in
> `/home/fbarroso/forest/loam` and **no token committed** anywhere (verified;
> `.env` is gitignored). The old "rotate the live creds in `loam/.env`" gate is
> **stale for this checkout** — treat it as a **pre-deploy verification**
> (confirm no `.env` with live tokens before building images; none present)
> rather than a hard blocker. loam injects creds at run time via `agentAuthEnv()`
> (`config.ts:313`).

Inherited on `phase/v7`: draft systems entries (`systems/mycelium.yaml`,
`systems/loam.yaml` — marked DRAFT) and both deployment plans.

> **UPDATE 2026-07-19 — lab-side onboarding landed; specs re-reconned
> 2026-07-20.** The steps that live in *this* repo are done and statically
> verified: **KM-3** (`guests/mycelium/` + the `mycelium` broker vhost in
> `generate-definitions.py`), **L-3** (`guests/loam/`), the EXP-70..74 scored
> defs, and the systems-entry notes. What remains is the **guest-side** work
> (KM-1/2/4, L-1/2) and every experiment *run* (no live cluster/AWS). The guest
> repos are **present and were re-reconned** (not absent, as an earlier draft
> assumed), so the specs below are corrected against verified upstream state.
> The honest ledger of what's left, with entry points, is
> [v7-DEFERRED.md](v7-DEFERRED.md). The KM/loam sections below are the
> guest-side spec — still the source of truth for those PRs.

## KM track

**KM-1 (mycelium repo) — fake-LLM mode (ADR-007.3, contract gate) —
GREENFIELD:** there is **no `LLM_MODE` and no fake-LLM path anywhere in
mycelium today**; the existing `dry_run` flag only skips DB *writes* and still
spends on OpenAI/Voyage, so it is not a substitute. Central seams are the LLM
client `src/lib/llm/client.py` (real path today: OpenAI, raises if
`OPENAI_API_KEY` unset; `GRAPHRAG_MODEL` default `gpt-4o-mini` at
`src/core/config/graphrag.py:29`) and the Voyage embedder
`src/domain/ingestion/stages/embed.py` (`VOYAGE_RPM=20`, the throughput
bottleneck, is read at `src/infrastructure/llm/rate_limit.py:15`; embeddings
dim 1024). Spec: `LLM_MODE=fake` env →
clients return deterministic fixtures keyed by `sha256(model + prompt)[:16]`
from `fixtures/llm/` (committed, generated once with real keys via a `record`
mode: `LLM_MODE=record` writes through while capturing); embedder returns
`sha256`-seeded pseudo-vectors (dim 1024) so downstream similarity is stable.
Fake mode must hard-fail if `OPENAI_API_KEY`/`VOYAGE_API_KEY` are SET (proves
zero-spend by construction — EXP-70's "no key mounted" assertion inverts to
"fake mode refuses keys"). Config flag surfaced in `.env` + README. *(All of
the above is the KM-1 spec — none of it exists upstream yet.)*

**KM-2 (mycelium repo) — containerize — GREENFIELD** (no Dockerfiles exist;
CI builds no images): the primary API surface is now a **consolidated
FastAPI/uvicorn app** that binds **both `:8080` (stages) and `:8081` (graph)
in one process** (`src/app/api/fastapi_app/main.py:266-278`) — so **one
GraphRAG image**, not a stages-api + graph-api split, plus a **StagesUI**
image. **No GraphDash image** — GraphDash was **retired upstream in mycelium's
phase 4** (no `:3001`, no directory). (The legacy standalone `http.server`
stages/graph entrypoints still exist as secondary CLIs but are not the primary
surface.) Multi-stage, non-root 10001. `/health` **already exists**
(`fastapi_app/main.py:179`) and `/metrics` is already served (`main.py:155`,
Prometheus text); **`/ready` is the real gap** — add it.

**KM-3 (lab repo) — onboarding:** `guests/mycelium/` per HOST_CONTRACT
(compose project on the 42xx block; k8s namespace `mycelium`,
netpols default-deny + egress mongo.lab-infra:27017 + rabbitmq:5672 +
lab-obs scrape ingress; NO egress to api.openai.com in fake mode — that's
the netpol-level spend proof). *(These manifests are provisional and predate
the 2026-07-20 recon — they over-decompose the app: GraphDash is retired and
the two APIs are one consolidated FastAPI image, so the component/port set
shrinks when KM-2's real images land; YAML is reconciled elsewhere.)* Topology:
extend `scripts/rabbitmq/generate-definitions.py` with a `mycelium` vhost
(stage queues `km.stage.<name>` + retry tiers + DLQ, same generator
pattern; ADR-007.4) — definitions stay single-sourced. ServiceMonitor +
a KM Grafana dashboard built on the **real, already-served** `/metrics`
names — **unprefixed** `chunks_processed` (labels `stage`/`stage_name`) and
`stage_duration_seconds` (`src/lib/metrics/`); there is **no `mycelium_*`
prefix**. launch.yaml; un-DRAFT `systems/mycelium.yaml`.

**KM-4 (mycelium repo) — stage-level queue-mode:** `PipelineRunner
--queue-mode` per the deployment plan §2 (stage boundaries → lab RabbitMQ
envelopes on the vhost; in-process default untouched upstream). **Not the same
as what exists:** mycelium already runs a **real arq/Redis queue at the RUN
level** (phase-3, `src/app/workers/`) that enqueues whole pipeline runs and
falls back to an in-process thread if Redis is absent — KM-4 is the *separate,
future* migration of **stage boundaries** onto RabbitMQ (ADR-007.4), not that.
Today stages run in-process sequentially over a shared `PipelineRunner` with
**MongoDB collections as the stage handoff**. Real stage order to preserve —
ingestion: `ingest → clean → chunk → enrich → embed → redundancy → trust`
(**chunk before enrich**); graphrag: `graph_extraction → entity_resolution →
graph_construction → community_detection → insights_generation`
(`insights_generation` is new in mycelium's own phase-7). Audit stage
idempotency first (plan's open question — upsert-shaped writes are the
precondition for retry tiers).

**Exit:** EXP-70 (fixture flood in fake mode, stages drain on its
dashboard, retry/DLQ per conventions, zero spend proven) and EXP-71 (one
budgeted real-key run; record $ + fake-vs-real stage timings).

## loam track

**L-1 (loam repo) — callsite indirection:** introduce one seam (e.g.
`createRunnerSandbox()`) and route the **8 files / 9 `docker()` call sites**
through it — `implement-issue`, `orchestrate/integrate`, **`orchestrate/
index.ts` (TWO calls @2070 and @2179 — the main engine, missed by the "~7
files" count)**, `review-pr`, `wave`, `distill-lessons`, `ingest-corpus`,
`ingest-v1`. **Also** account for the host-side `noSandbox` path in
`orchestrate/operator.ts:662` (non-docker). There is no central factory today
(loam PRD.md:119: "docker(...) is hardcoded at every sandbox site"), and
sandcastle's `SandboxProvider` tagged-union has **0 references in loam**, so the
indirection is genuinely required. The isolated handle surface to preserve is
`exec` / `interactiveExec?` / `copyIn` / `copyFileOut` / `close` /
`worktreePath` — note there is **no `run()` on the handle** (`run` is a
top-level sandcastle function). `loam.config.json` gains `runner: docker|k8s`.
Pure refactor, its own PR, `--dry-run` output unchanged.

**L-2 (loam repo) — k8s-Jobs provider (ADR-007.5):** implement the isolated
handle contract per the deployment plan's adapter spec. **Resolve the
execution-model split FIRST** — this is the deepest gap: loam today is
**host-driven** (the host holds the sandbox handle and drives the agent via
streamed `exec` while the container just runs `sleep infinity`, per
`sandbox/polyglot.Dockerfile`), whereas the example agent-Job is
**fire-and-forget autonomous** (`sh -c "loam-agent run && git push"`) — a
*different* model, and **there is no `loam-agent` binary in the image today**.
The adapter must either keep **host-driving the pod** via `kubectl exec`/API
(fits the sandcastle seam) **or invert to an autonomous in-pod entrypoint**
(new work). Then: branch pushed to origin on completion, host reads
`branchAheadShas` against `origin/<branch>` (this part is correct as-is;
`copyFileOut` also exists for pulling artifacts and is currently unused). Job
spec knobs — resources from `repos.<area>.k8s.*`, `activeDeadlineSeconds` from
`runTimeoutMs`, `ttlSecondsAfterFinished`, `backoffLimit: 0` — **none of which
exist today** (loam has only software timeouts: setup hook 120s, verify
concurrency 1, vitest caps). Token via `secretKeyRef` (ADR-007.6), sourced the
way loam already does — `agentAuthEnv()` (`config.ts:313`) reads
`CLAUDE_CODE_OAUTH_TOKEN` then `ANTHROPIC_API_KEY`. Dry-run renders the
manifest; preflight gains a `kubectl auth can-i create jobs` probe.

**L-3 (lab repo) — onboarding:** `guests/loam/` (UI + control API as one
deployment; namespace `loam`; ResourceQuota/LimitRange for 3 concurrent agent
Jobs; Secret `loam-agent-token`; netpols with the documented egress holes to
api.anthropic.com:443 + github.com:443). **Ports:** loam's **internal** ports
are **4400 (UI)** and **4500 (control)**; the 43xx (4310→4400, 4320→4500) are
only the **host-side compose mapping**, and the k8s Services correctly target
4400/4500 — don't claim loam "listens on 43xx". **`host=0.0.0.0` is a no-op:**
loam does **not** read `process.env.HOST` — `serve` binds non-loopback only via
a `--host` flag and `loam ui` has **no** host flag (always loopback), so the
launch commands must pass `--host 0.0.0.0` (and loam needs a new `loam ui
--host`) or the containers bind loopback and are unreachable. **Readiness
probe:** `/api/capabilities` is on the control port 4500, which is
**loopback-only enforced** (`serve.ts:535-537` 403s any non-loopback `Host`),
so a kubelet probe gets 403'd and the pod never goes Ready — **probe the UI
(4400, no such guard) instead**, or gate on a future non-loopback/bearer serve
mode (loam PRD lists remote-serve unbuilt). launch.yaml; un-DRAFT
`systems/loam.yaml`.

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
