# loam deployment plan — v7 draft ("agent farm")

**Repo:** `~/repo/forest/loam` (phase doc's `~/repo/Raine/loam` is stale).
Recon 2026-07-19.

> ⚠️ **Before any deployment work:** `loam/.env` holds a live
> `CLAUDE_CODE_OAUTH_TOKEN` and `ANTHROPIC_API_KEY` in plaintext
> (gitignored but real). Rotate them and move to the lab secret path
> (ADR-007.6) before building images or running agents in the lab.

## What it actually is (recon summary)

npm-workspaces TS monorepo, Node ≥20: `core` (loam CLI), `workflows` (the
execution engine — the load-bearing package), `ui` (Hono+React knowledge
UI, read-only GETs, **:4400**), `plugin`. Agent sandboxes come from the
external **`@ai-hero/sandcastle` v0.12.0**: loam imports the concrete
`docker()` provider factory directly in ~7 workflow files
(`implement-issue`, `orchestrate/integrate`, `review-pr`, `wave`,
`distill-lessons`, `ingest-corpus`, `ingest-v1`); model binding in
`packages/workflows/src/shared/agent.ts`. Sandbox = `loam-sandbox:<area>`
image (node:22-bookworm + Go/Python + Claude Code CLI, `sleep infinity`
entrypoint) with a **host git worktree bind-mounted**; auth injected via
sandbox provider env (`agentAuthEnv()` in `config.ts`, OAuth token
preferred). Artifacts (commits/branch/diff) are read **host-side** via
local git (`branchAheadShas`) then pushed/PR'd from the host. Every
workflow has `--dry-run`. Orchestrate control API on **127.0.0.1:4500**
(`GET /api/capabilities` is the closest health probe). No /metrics
anywhere. Docker cpu/mem caps are not surfaced by loam today.

## Production target shape ("agent farm")

Queue of runs → **Kubernetes Jobs on EKS** → logs + artifacts → knowledge
updates. Long-running pieces: knowledge UI (:4400) + orchestrate control
API (:4500) as one deployment (both Hono servers, loopback-default —
containerize with host=0.0.0.0). Each agent run = one Job: sandbox image,
resource limits, `activeDeadlineSeconds` (wall-clock budget),
`ttlSecondsAfterFinished`, token from a k8s Secret (→ Secrets Manager on
AWS, ADR-007.6), logs captured by the lab's fluent-bit.

## The k8s-Jobs runner adapter (loam-side; spec, ADR-007.5)

The seam already exists in sandcastle (`SandboxProvider` tagged union;
`IsolatedSandboxHandle`: `exec/run/close/worktreePath`). Two structural
constraints found in recon shape the design:

1. **Callsite indirection first:** loam imports `docker()` concretely in 7
   files — introduce `createRunnerSandbox()` in
   `packages/workflows/src/shared/agent.ts` (backend chosen by
   `loam.config.json` `runner: docker|k8s`), route all callsites through
   it. This is a pure refactor PR that lands before any k8s code.
2. **The host-worktree assumption is the hard part:** artifacts are read
   from the local git worktree after the run. A k8s Job has no host
   worktree. Adapter contract: the Job clones the repo + branch inside the
   pod (init container), the agent works there, and on completion the Job
   **pushes the branch to origin**; the loam host then fetches and reads
   `branchAheadShas` against `origin/<branch>` instead of the local
   worktree. (Requires a scoped deploy key/secret; gh PR creation stays
   host-side, unchanged.)
3. Job spec knobs (per run): image, `resources` (limits from a new
   `repos.<area>.k8s.{cpu,memory}` config), `activeDeadlineSeconds` from a
   new `runTimeoutMs`, `ttlSecondsAfterFinished: 3600`, `backoffLimit: 0`
   (retries are loam's decision, not the Job controller's), namespace
   `loam`, labels `loam.run-id`. Token via `secretKeyRef` →
   `CLAUDE_CODE_OAUTH_TOKEN`. Log capture: stdout (fluent-bit picks it up)
   + the existing file logging into an `emptyDir` uploaded on completion
   (or dropped — stdout is authoritative in the lab).
4. `--dry-run` parity: k8s backend renders the Job manifest and prints it
   without applying (the existing dryRun path short-circuits earlier;
   extend it to show the manifest).
5. Preflight (`shared/preflight.ts`): add a k8s probe (kubectl auth
   can-i create jobs -n loam) mirroring the docker-daemon check.

## Lab rehearsal scope (v7)

`guests/loam/`: namespace `loam`, knowledge-UI+control-API deployment
(port block 43xx: 4310 UI, 4320 control), ResourceQuota/LimitRange sized
for N=3 concurrent agent Jobs, netpols (Jobs need egress to
api.anthropic.com:443 + github.com:443 — a deliberate, documented hole in
default-deny), Secret `loam-agent-token`. Experiments: EXP-72 (lifecycle +
hung-agent hits activeDeadline), EXP-73 (OOM at limit without disturbing
neighbors; token-absent fails fast; rotation drill).

## Open questions

- Sandcastle upstream vs loam-side provider: implement the k8s provider
  inside loam (wrapping the sandcastle types) first; upstream later if it
  stabilizes (avoids forking sandcastle now).
- Multi-arch images (lab kind on arm64 Mac vs EKS amd64) — build both or
  pin the drill to one target.
- Whether the control API drives Job launches in the lab rehearsal or the
  CLI does (start with CLI; Mission Control integration is out of scope,
  ADR-005.5).
