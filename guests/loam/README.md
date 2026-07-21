# loam

The **agent-farm** guest: loam is a TypeScript npm-workspaces monorepo (Node
≥20) that launches Claude Code agents to do software work. This directory
onboards loam's long-running control plane into the lab per the host contract
([`documentation/HOST_CONTRACT.md`](../../documentation/HOST_CONTRACT.md)) and
scaffolds the isolation/quota/secret/netpol envelope its **agent runs** need.
Recon + architecture:
[`documentation/deployment/LOAM_DEPLOYMENT_PLAN.md`](../../documentation/deployment/LOAM_DEPLOYMENT_PLAN.md).

> ⚠️ **SECURITY — verify no live token before building images (pre-deploy check).**
> **This checkout has no `.env` file** (`/home/fbarroso/forest/loam`; verified —
> `stat`, `find`, `git status --ignored` all show absence) and **no token is
> committed anywhere** (`git grep` clean; the only `sk-ant-` string is the
> validator regex at `config.ts:318`). `.env` is correctly gitignored and
> untracked. So the earlier "step-0: rotate the live token" gate is **not
> applicable to this checkout** — it downgrades to a **verification** step:
> confirm no `.env` with live tokens exists before building images (none present
> now), then inject the token via the lab secret path (ADR-007.6). loam reads
> `CLAUDE_CODE_OAUTH_TOKEN` (then `ANTHROPIC_API_KEY`) via `agentAuthEnv()`
> (`packages/workflows/src/config.ts:313`). The Secret in `k8s/base/secret.yaml`
> is a clearly-fake placeholder; never commit a real token to this tree.

## What this guest is

Two long-running servers, deployed as **one** pod / one container
(`localhost:5001/loam:dev`):

| Component     | Container port | Compose host | Purpose                                             |
|---------------|----------------|--------------|-----------------------------------------------------|
| knowledge-ui  | 4400           | **4310**     | React 19 + Vite client / Hono server; read-only GETs, no auth |
| control-api   | 4500           | **4320**     | orchestrate control API (`serve`)                   |

Ports **4400/4500 are loam's internal ports**; the **43xx** block is only the
host-side compose mapping (`4310→4400`, `4320→4500`) — the k8s Services use
4400/4500 directly. loam does **not** listen on 43xx.

> ⚠️ **`HOST=0.0.0.0` is a no-op today (guest-side gap).** loam does not read
> `process.env.HOST`. `serve` (control-api) binds non-loopback only via a
> `--host` flag; `loam ui` (knowledge-ui) has **no** host flag and *always*
> binds loopback. So the `HOST=0.0.0.0` that compose/k8s set is ignored — the
> containers keep binding `127.0.0.1` and stay unreachable. The loam image must
> add `HOST` support (or the launch commands must pass `--host 0.0.0.0`, plus a
> new `loam ui --host`) before the pod is reachable.
>
> The same loopback enforcement breaks the **readiness probe**: `GET
> /api/capabilities` on the control port (4500) is loopback-only — its auth
> middleware 403s any non-loopback `Host` (`serve.ts:535-537`) — so a kubelet
> probe is rejected and the pod never goes Ready. Probe the **UI** (4400, no
> such guard) instead, or gate readiness on loam shipping a non-loopback/bearer
> serve mode (unbuilt — `PRD.md:126`).

**Agent runs are NOT a long-running service** — and today they are
**host-driven**: the host process holds the sandbox handle and drives the Claude
Code agent via streamed `exec` while the container just runs `sleep infinity`
(`sandbox/polyglot.Dockerfile`). Execution is delegated to external
`@ai-hero/sandcastle`. Artifacts/logs/PR are already read host-side (git
worktree, `gh`) — that part of the plan is correct.

The lab's **target** is one **Kubernetes Job per run** in namespace `loam`
(`runner: k8s`), with resource limits, `activeDeadlineSeconds`,
`ttlSecondsAfterFinished`, `backoffLimit: 0`, and the token via `secretKeyRef`.
**This is aspirational** — no k8s/Jobs code exists in loam yet (`PRD.md:118`),
and those resource/TTL knobs are *new* (loam today has only software timeouts).
`k8s/base/agent-job.example.yaml` is reference only.

> ⚠️ **Execution-model split (the deepest L-2 gap).** The example Job describes
> a **fire-and-forget autonomous** pod (`sh -c "loam-agent run && git push"`) —
> a *different* model from today's host-driven one, and **there is no
> `loam-agent` binary in the image**. The k8s-Jobs adapter must first resolve
> this: keep host-driving the pod via `kubectl exec`/API (fits the sandcastle
> seam) **or** invert to an autonomous in-pod entrypoint (new work).

## Guest-side pending (this is scaffolding)

The loam repo **is present** (`/home/fbarroso/forest/loam`, git `main`), but
this lab-side onboarding still depends on two guest-side deliverables:

- **The container image `localhost:5001/loam:dev`** — built + pushed guest-side.
  There is no Dockerfile or build context here; compose/kind reference the tag
  only. `up` will not pull until the image exists. loam must also add `HOST`
  support (above), or the launch commands stay loopback-bound.
- **The k8s-Jobs runner adapter** (guest-side PRs **L-1** callsite indirection /
  **L-2** k8s provider, ADR-007.5). Today `docker(...)` is hard-wired at **9
  call sites across 8 files** with no central factory (`PRD.md:118`);
  `operator.ts` also uses `noSandbox` (host-side) that L-1 must account for. The
  `SandboxProvider` tagged-union seam lives in sandcastle and loam references it
  **0 times**, so the L-1 indirection is genuinely required. **L-2** must then
  resolve the execution-model split above.

A lab-side apply of `k8s/base` stands up the namespace, the (unpullable-yet)
Deployment, netpols, quota, placeholder Secret, and ingress — everything except
a working image and a live agent runner.

## 5-minute tour

```bash
# 0. Lab up (creates microservices_default + the obs stack):
make up                                                   # repo root

# 1. Compose (needs localhost:5001/loam:dev pushed guest-side first, AND the
#    HOST/reachability gap resolved — see "What this guest is"):
docker compose -f guests/loam/docker-compose.yml up -d
curl -s localhost:4310/                                   # readiness via UI (control API is loopback-only)
open  http://localhost:4310/                              # knowledge UI

# 2. kind (image must already be pushed to localhost:5001):
kubectl apply -k guests/loam/k8s/base
kubectl -n loam rollout status deploy/loam --timeout=120s
#    then https://loam.lab.local/  (add to /etc/hosts or curl --resolve)

# Inspect without applying:
kustomize build guests/loam/k8s/base | less
```

## Shared infra / secret mapping (ADR-007.1 / ADR-007.6)

- **Shared infra:** loam needs **no** shared postgres/mongo/minio/rabbitmq in
  the lab rehearsal — its state is the git repos it operates on and the
  knowledge docs. Its one external dependency is the **agent token Secret**
  (`loam-agent-token`, key `CLAUDE_CODE_OAUTH_TOKEN`), consumed by agent Jobs
  via `secretKeyRef`. In the lab the value is placeholder; in production it maps
  to **AWS Secrets Manager** (ADR-007.6). Agent Jobs also reach the public
  internet — the one documented exception to default-deny (see below).
- **Isolation (ADR-007.2):** own compose project (`name: loam`) + own default
  network; k8s namespace `loam` (`lab.local/tier: guest`, `lab.local/guest:
  loam`) with default-deny netpols.
- **Observability:** loam exposes **no `/metrics`** today (recon). The obs
  overlay (`k8s/obs/`) is a kustomize-valid placeholder; the scrape netpol is
  kept in the pattern for when a `loam_...` endpoint exists.

## Contract evidence

| Contract clause (HOST_CONTRACT.md) | Evidence here |
|---|---|
| §1.1 containerized, pinned images | image `localhost:5001/loam:dev` referenced (built guest-side, PRs L-1/L-2) — no Dockerfile in this lab-side tree by design |
| §1.2 /health /ready /metrics | control-API `/api/capabilities` is **loopback-only** (`serve.ts:535-537`) so a kubelet probe 403s — probe the UI (4400) or gate on a future non-loopback serve mode; **no `/metrics`** — obs overlay is a placeholder |
| §1.3 `launch.yaml` | [`launch.yaml`](launch.yaml) — compose + kind up/down/status, two components, ports |
| §1.4 assigned port block | 43xx is host mapping only: knowledge-ui 4310→4400, control-api 4320→4500 (loam's internal ports are 4400/4500) |
| §1.6 deployment note | below + LOAM_DEPLOYMENT_PLAN |
| §2 isolation (host side) | compose project `loam` + own default network; `k8s/base` namespace `loam`, default-deny netpols; ResourceQuota + LimitRange for N=3 agents |
| §2 observability (host side) | compose alias `loam` on `microservices_default`; `k8s/obs/` placeholder ServiceMonitor (no metrics yet) |
| §2 ingress (host side) | `k8s/base/ingress.yaml` — `loam.lab.local` → knowledge UI, TLS via `lab-ca` |

## NetworkPolicy: the deliberate egress hole (ADR-007.5/.6)

Every other guest is fully egress-locked. loam is the exception: its **agent
Jobs must reach the internet** — `api.anthropic.com:443` (the model) and
`github.com:443` (clone/push). `k8s/base/netpols.yaml` opens **TCP/443 to
`0.0.0.0/0`** (cluster/service CIDRs `except`ed back out) **only** for pods
labelled `app: loam-agent`. The always-on UI/control pod does **not** carry that
label and keeps DNS-only egress.

Why a CIDR and not the two hostnames: NetworkPolicy `egress.to` matches CIDRs
and label selectors, **never FQDNs** — it cannot express "allow
api.anthropic.com". FQDN-scoped egress (Cilium `toFQDNs` or an egress-proxy
allowlist) is a documented follow-up; the broad-port/narrow-protocol hole is the
accepted lab trade.

## Quota sizing (N=3 concurrent agent Jobs)

`k8s/base/quota.yaml`, per LOAM_DEPLOYMENT_PLAN (N=3):

- **per agent:** request `500m`/`1Gi`, limit `2 CPU`/`4Gi`
- **UI/control:** request `250m`/`256Mi`, limit `500m`/`512Mi`
- **ResourceQuota totals** (3 agents + UI): `requests.cpu 2`, `requests.memory
  4Gi`, `limits.cpu 7`, `limits.memory 13Gi`, `pods 6`
- **LimitRange** sets those per-agent limits as container defaults, so an agent
  Job that omits limits still lands bounded — and a 4th agent (or an
  over-limit one) is rejected/killed by the quota, not by starving the UI
  (EXP-73's neighbour-isolation proof).

## Deployment note (contract §1.6)

**How this deploys for real:** the "agent farm" — a queue of runs → Kubernetes
Jobs on **EKS** → logs + artifacts → knowledge updates (LOAM_DEPLOYMENT_PLAN).
The always-on UI/control deployment is unchanged between lab and prod except
registry, hostname, and replica count. **Lab vs production differences:** token
from a placeholder Secret → AWS Secrets Manager; image from `localhost:5001` → a
real registry; egress from a broad `0.0.0.0/0:443` netpol → an FQDN-scoped
policy. **What it still lacks for production:** the execution-model split
(host-driven vs autonomous in-pod, above) and the k8s-Jobs adapter (L-1/L-2) are
unbuilt; loam ignores `HOST` so the image/launch must add reachability; no
`/metrics` (no dashboards or alerting on loam itself yet); FQDN egress not
implemented; multi-arch images (kind on arm64 vs EKS amd64) unresolved. Honest
status: **scaffolding**, gated on the guest-side image + adapter — plus a
pre-deploy check that no `.env` with live tokens exists (none in this checkout).
