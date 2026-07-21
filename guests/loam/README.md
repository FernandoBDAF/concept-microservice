# loam

The **agent-farm** guest: loam is a TypeScript npm-workspaces monorepo (Node
≥20) that launches Claude Code agents to do software work. This directory
onboards loam's long-running control plane into the lab per the host contract
([`documentation/HOST_CONTRACT.md`](../../documentation/HOST_CONTRACT.md)) and
scaffolds the isolation/quota/secret/netpol envelope its **agent runs** need.
Recon + architecture:
[`documentation/deployment/LOAM_DEPLOYMENT_PLAN.md`](../../documentation/deployment/LOAM_DEPLOYMENT_PLAN.md).

> ⚠️ **SECURITY — rotate the live token first (step 0, guest-side).**
> `loam/.env` in the loam repo holds a **live `CLAUDE_CODE_OAUTH_TOKEN` and
> `ANTHROPIC_API_KEY` in plaintext** (gitignored but real, per
> LOAM_DEPLOYMENT_PLAN). **Rotate them and move to the lab secret path
> (ADR-007.6) BEFORE building images or running any real agent in the lab.**
> The Secret in `k8s/base/secret.yaml` here is a clearly-fake placeholder — the
> real token is injected out-of-band by init-secrets / AWS Secrets Manager.
> Never commit a real token to this tree.

## What this guest is

Two long-running Hono servers, deployed as **one** pod / one container
(`localhost:5001/loam:dev`):

| Component     | Container port | Compose host | Purpose                                   |
|---------------|----------------|--------------|-------------------------------------------|
| knowledge-ui  | 4400           | **4310**     | Hono+React knowledge browser (read-only)  |
| control-api   | 4500           | **4320**     | orchestrate control API (`/api/capabilities` = readiness) |

Both default to loopback in the loam repo; the container binds `host=0.0.0.0`
(via the `HOST` env). Port block **43xx** (HOST_CONTRACT §1.4).

**Agent runs are NOT a long-running service.** Each run executes as a
**Kubernetes Job in namespace `loam`** — scheduled by the loam-side k8s-Jobs
adapter (`runner: k8s`), one Job per run, with resource limits,
`activeDeadlineSeconds`, `ttlSecondsAfterFinished`, `backoffLimit: 0`, and the
token via `secretKeyRef`. They are **not** a compose service and **not** a
registry image the lab builds. See `k8s/base/agent-job.example.yaml` for the
concrete envelope (reference only — the adapter renders real Jobs).

## Guest-side pending (this is scaffolding)

This is a **lab-side** onboarding only. Two things it depends on come from the
loam repo, which is **not yet onboarded**:

- **The container image `localhost:5001/loam:dev`** — built + pushed guest-side.
  There is no Dockerfile or build context here; compose/kind reference the tag
  only. `up` will not pull until the image exists.
- **The k8s-Jobs runner adapter** (guest-side PRs **L-1** callsite indirection /
  **L-2** k8s provider, ADR-007.5) — the code that renders and applies agent
  Jobs. Until it lands, the example Job is illustrative, not driven.

A lab-side apply of `k8s/base` stands up the namespace, the (unpullable-yet)
Deployment, netpols, quota, placeholder Secret, and ingress — everything except
a working image and a live agent runner.

## 5-minute tour

```bash
# 0. Lab up (creates microservices_default + the obs stack):
make up                                                   # repo root

# 1. Compose (needs localhost:5001/loam:dev pushed guest-side first):
docker compose -f guests/loam/docker-compose.yml up -d
curl -s localhost:4320/api/capabilities                   # control API readiness
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
| §1.2 /health /ready /metrics | readiness/liveness on control API `/api/capabilities` (recon: closest probe); **no `/metrics`** — noted, obs overlay is a placeholder |
| §1.3 `launch.yaml` | [`launch.yaml`](launch.yaml) — compose + kind up/down/status, two components, ports |
| §1.4 assigned port block | 43xx: knowledge-ui → 4310, control-api → 4320 (only host ports published) |
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
policy. **What it still lacks for production:** no `/metrics` (no dashboards or
alerting on loam itself yet); FQDN egress not implemented; the k8s-Jobs adapter
(L-1/L-2) is guest-side and not yet merged; multi-arch images (kind on arm64 vs
EKS amd64) unresolved. Honest status: **scaffolding**, gated on the guest-side
image + adapter and the step-0 token rotation.
