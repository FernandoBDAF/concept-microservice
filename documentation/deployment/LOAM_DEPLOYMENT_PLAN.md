# loam deployment plan — v7 draft ("agent farm")

**Repo:** `/home/fbarroso/forest/loam` — the loam checkout is **present** as a
sibling under the shared `forest/` root (git `main`), and was re-reconned for
this pass. Earlier drafts cited `~/repo/forest/loam` / `~/repo/Raine/loam` (and
`~/repo/mycelium`, `~/repo/forest/mycelium` for the sibling guest); **all such
`~/repo/...` paths are stale** — the real checkouts are
`/home/fbarroso/forest/<name>` (`/home/fbarroso/forest/loam`,
`/home/fbarroso/forest/mycelium`). The prior v7 pass assumed the guest repos
were absent and deferred guest-side work as un-startable; that premise is false
— they are present, so this plan distinguishes (a) what loam already has
upstream, (b) what is genuinely still TODO guest-side, and (c) lab-side
scaffolding that stays provisional until guest images exist.
Recon 2026-07-20 (re-verified; supersedes the 2026-07-19 pass).

> **Pre-deploy secret check (verify, don't assume — not a blocker for this
> checkout):** as of this recon **there is no `.env` in
> `/home/fbarroso/forest/loam`** — verified by `stat`, `find`, and
> `git status --ignored` (all show absence), and **no token is committed
> anywhere** (`git grep` is clean; the only `sk-ant-` string in the tree is the
> validator regex at `config.ts:318`). `.env` is correctly gitignored and
> untracked, so ignore-hygiene is sound. The earlier "Step 0: rotate the live
> `CLAUDE_CODE_OAUTH_TOKEN`/`ANTHROPIC_API_KEY` in `loam/.env`" gate was **stale
> for this checkout** and is downgraded from a hard blocker to a **pre-deploy
> verification step**: before building images or running agents in the lab,
> confirm no `.env` with live tokens is present (none is here) and that
> credentials flow only through the lab secret path (ADR-007.6), never a
> committed or image-baked file.

## What it actually is (recon summary)

npm-workspaces TS monorepo, Node ≥20: `core` (loam CLI), `workflows` (the
execution engine — the load-bearing package), `ui` (knowledge UI —
`@fernandobdaf/loam-ui`, React 19 + Vite 6 client + Hono server, read-only GETs,
no auth, **:4400**, binds `127.0.0.1` by default), `plugin`. Agent sandboxes come
from the external **`@ai-hero/sandcastle` v0.12.0**: loam imports the concrete
`docker()` provider factory directly at **8 files / 9 call sites** (earlier draft
said "~7 files" and **missed the main engine**): `implement-issue`,
`orchestrate/integrate`, **`orchestrate/index.ts` — the orchestrate engine, with
TWO calls at `:2070` and `:2179`**, `review-pr`, `wave`, `distill-lessons`,
`ingest-corpus`, `ingest-v1`. There is **no central factory** — `docker(...)` is
hard-wired at each site (loam PRD.md:119: "today `docker(...)` is hardcoded at
every sandbox site"). Separately, `orchestrate/operator.ts:662` uses
**`noSandbox({ env: agentAuthEnv() })`** (host-side, non-docker) — the L-1
refactor must route it too. Model binding in
`packages/workflows/src/shared/agent.ts`. Sandbox = `loam-sandbox:<area>`
image (node:22-bookworm + Go/Python + Claude Code CLI, `sleep infinity`
entrypoint — `sandbox/polyglot.Dockerfile`) with a **host git worktree
bind-mounted**. Auth: `agentAuthEnv()` (`config.ts:313`) reads
`CLAUDE_CODE_OAUTH_TOKEN` (validated against `^sk-ant-oat\d{2}-...`,
`config.ts:318`) then falls back to `ANTHROPIC_API_KEY`, injected into the
sandbox provider env; the `.env` itself is read by a hand-rolled `loadDotEnv`
(`config.ts:210`) from the **governed workspace's config dir**, not the loam repo
root, and there is no `dotenv` package. Artifacts (commits/branch/diff) are read
**host-side** via local git (`branchAheadShas` in `implement-issue/index.ts`;
`shared/git.ts` has `branchAheadCount`/`detectBaseDrift`), logs are
host-side, and the PR is created host-side via `gh` — this part of the plan is
correct and unchanged. Every workflow has `--dry-run`. Orchestrate control API on
**127.0.0.1:4500** — but that port is **loopback-only enforced**: its auth
middleware `403`s any non-loopback `Host` header (`serve.ts:535-537`), so
`GET /api/capabilities` is **not** a usable kubelet health probe (see the adapter
section). No `/metrics` anywhere. On resource controls loam has **only software
timeouts today** (setup hook 120s, verify concurrency 1, vitest caps) — **no
container cpu/mem limits, no `activeDeadlineSeconds`, no
`ttlSecondsAfterFinished`**; those are all NEW knobs the k8s Job would introduce.

## Production target shape ("agent farm")

Queue of runs → **Kubernetes Jobs on EKS** → logs + artifacts → knowledge
updates. Long-running pieces: knowledge UI (:4400) + orchestrate control
API (:4500) as one deployment (both Hono servers, loopback-default). **Caveat —
`HOST=0.0.0.0` is a no-op today:** loam does **not** read `process.env.HOST`
(0 references); `serve` binds non-loopback only via an explicit `--host` flag,
and `loam ui` has **no** host flag at all (always loopback). So the
`HOST=0.0.0.0` that compose/k8s set is ignored — the containers keep binding
loopback and stay unreachable. The **guest-side loam image must add `HOST`
support** (or the launch commands must pass `--host 0.0.0.0`, plus a new
`loam ui --host`) before either server is reachable in-cluster; this is unbuilt
today. Each agent run = one Job: sandbox image, resource limits,
`activeDeadlineSeconds` (wall-clock budget), `ttlSecondsAfterFinished`, token
from a k8s Secret (→ Secrets Manager on AWS, ADR-007.6), logs captured by the
lab's fluent-bit.

## The k8s-Jobs runner adapter (loam-side; spec, ADR-007.5)

The `SandboxProvider` tagged-union seam lives **in sandcastle**, and loam does
**not** reference it (`grep` shows 0 uses) — so the L-1 indirection below is
genuinely required, not already present. The isolated handle surface is
`exec` / `interactiveExec?` / `copyIn` / `copyFileOut` / `close` / `worktreePath`
— there is **no `run()` method on the handle** (`run` is a top-level sandcastle
function, not a handle method); `copyFileOut` exists for pulling artifacts out of
an isolated sandbox and is currently **unused** by loam, but would be useful to a
k8s adapter. Three structural constraints found in recon shape the design — the
first is the deepest and must be resolved before the others:

1. **Execution model — HOST-DRIVEN vs fire-and-forget (resolve this FIRST):**
   loam today is **host-driven** — the host process holds the sandbox handle and
   drives the agent via **streamed `exec`** while the container just runs
   `sleep infinity` (`sandbox/polyglot.Dockerfile`). The current
   `agent-job.example.yaml` + earlier plan describe a **fire-and-forget
   autonomous Job** (`sh -c "loam-agent run && git push"`) — a **different
   execution model**, and **there is no `loam-agent` binary in the image today**
   (0 such code exists; the whole k8s/Jobs path is aspirational, loam
   PRD.md:118). The k8s-Jobs adapter (L-2) must first pick a side: **(A) keep
   host-driving the pod** via `kubectl exec` / the k8s API (fits the existing
   sandcastle handle seam — the host still holds `exec`/`copyFileOut`), or
   **(B) invert to an autonomous in-pod entrypoint** (a new `loam-agent` binary
   plus push-on-completion — new guest-side work). This split subsumes the
   host-worktree question below and is the single most important design decision
   here.
2. **Callsite indirection (L-1, pure refactor, lands first):** loam imports
   `docker()` concretely at **8 files / 9 call sites** — including the two calls
   in `orchestrate/index.ts` (`:2070`, `:2179`) — plus the `noSandbox` site in
   `orchestrate/operator.ts:662`. Introduce `createRunnerSandbox()` in
   `packages/workflows/src/shared/agent.ts` (backend chosen by
   `loam.config.json` `runner: docker|k8s`) and route **all nine docker sites
   and the operator's `noSandbox` site** through it. No central factory exists
   today, so this touches every site. This is a pure refactor PR that lands
   before any k8s code.
3. **The host-worktree assumption (only bites under model B):** artifacts are
   read from the local git worktree after the run (`branchAheadShas`). A k8s Job
   under **model B** has no host worktree. Adapter contract there: the Job clones
   the repo + branch inside the pod (init container), the agent works there, and
   on completion the Job **pushes the branch to origin**; the loam host then
   fetches and reads `branchAheadShas` against `origin/<branch>` instead of the
   local worktree (requires a scoped deploy key/secret; gh PR creation stays
   host-side, unchanged). Under **model A** the host keeps driving and can
   `copyFileOut`/read as today, so this constraint largely dissolves — another
   reason to settle constraint 1 first.
4. Job spec knobs (per run): image, `resources` (limits from a new
   `repos.<area>.k8s.{cpu,memory}` config), `activeDeadlineSeconds` from a
   new `runTimeoutMs`, `ttlSecondsAfterFinished: 3600`, `backoffLimit: 0`
   (retries are loam's decision, not the Job controller's), namespace
   `loam`, labels `loam.run-id`. Token via `secretKeyRef` →
   `CLAUDE_CODE_OAUTH_TOKEN`. All of these (cpu/mem, deadline, ttl) are **new** —
   loam has none of them today. Log capture: stdout (fluent-bit picks it up)
   + the existing file logging into an `emptyDir` uploaded on completion
   (or dropped — stdout is authoritative in the lab).
5. `--dry-run` parity: k8s backend renders the Job manifest and prints it
   without applying (the existing dryRun path short-circuits earlier;
   extend it to show the manifest).
6. Preflight (`shared/preflight.ts`): add a k8s probe (`kubectl auth
   can-i create jobs -n loam`) mirroring the docker-daemon check.

## Lab rehearsal scope (v7)

`guests/loam/`: namespace `loam`, knowledge-UI+control-API deployment. **Ports —
loam's internal ports are `4400` (UI) and `4500` (control), full stop.** The
"43xx block" is **only a host-side compose port-mapping** (`4310->4400`,
`4320->4500`); the **k8s Services correctly target `4400`/`4500`**, and loam does
not "listen on 43xx" anywhere. ResourceQuota/LimitRange sized for N=3 concurrent
agent Jobs, netpols (Jobs need egress to `api.anthropic.com:443` +
`github.com:443` — a deliberate, documented hole in default-deny), Secret
`loam-agent-token`. **Readiness-probe caveat:** do **not** probe
`GET /api/capabilities` on the control port (4500) — a kubelet probe carries a
non-loopback `Host` and gets `403`ed (`serve.ts:535-537`). Probe the **UI on
4400** (no such guard) instead, or gate pod-Ready on the future
non-loopback/bearer serve mode (unbuilt; loam PRD.md lists remote-serve as a
later-phase decision). **Lab manifests already correct vs loam reality** (leave
as is): the netpol agent-egress hole, `prometheus.io/scrape: false` + the
placeholder `ServiceMonitor` (loam ships no `/metrics` yet), the
ResourceQuota/LimitRange, and the placeholder `secret.yaml` value. Experiments:
EXP-72 (lifecycle + hung-agent hits activeDeadline), EXP-73 (OOM at limit without
disturbing neighbors; token-absent fails fast; rotation drill).

## Open questions

- **Execution model (blocking):** host-driven pod (model A, fits the sandcastle
  seam) vs autonomous in-pod `loam-agent` entrypoint (model B, new guest-side
  binary). Everything else in the adapter depends on this — decide it first.
- **Reachability/readiness (guest-side, unbuilt):** loam ignores `HOST` and both
  servers bind loopback; making the UI/control API reachable in-cluster needs a
  `--host 0.0.0.0` path (and `loam ui --host`), and making the control API
  probe-able/remotely usable needs the non-loopback/bearer serve mode loam has
  not built yet. Until then, probe the UI (4400) only.
- Sandcastle upstream vs loam-side provider: implement the k8s provider
  inside loam (wrapping the sandcastle types) first; upstream later if it
  stabilizes (avoids forking sandcastle now).
- Multi-arch images (lab kind on arm64 Mac vs EKS amd64) — build both or
  pin the drill to one target.
- Whether the control API drives Job launches in the lab rehearsal or the
  CLI does (start with CLI; Mission Control integration is out of scope,
  ADR-005.5).
