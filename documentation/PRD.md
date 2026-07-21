# PRD — Microservices Operations Lab

**Status:** living document · **Owner:** Fernando · **Last updated:**
2026-07-10 (all open questions decided — see
[decisions/](decisions/); execution briefs in [phases/](phases/))

## 1. Vision

A personal **operations practice platform**: a small but honest distributed
system (API, auth, queues, cache, object storage, async workers) that can be
stood up, broken, observed, and repaired on demand — locally first, then on
**real AWS infrastructure** — so that operating Kubernetes, cloud deployments,
and distributed infrastructure becomes muscle memory before it's needed on
real projects.

The lab is not only about its own built-in services. It is a **host platform**
able to incorporate other systems: the **Mission Control UI** chooses which
system/stack to launch, drives **guided experiments** against it, and gives
visibility over everything running. Two real guest systems are slated (§6): a
knowledge-manager (GraphRAG expansion) and an agentic-workflow-manager — the
goal is to plan their *real* deployments and rehearse them here first.

Its second output remains **generic, reusable infrastructure pieces** (auth
service, queue conventions + worker template, cache pattern, storage
pipeline) that future real projects lift wholesale.

**One-line test for scope decisions:** *"Does this make a realistic
operational scenario practicable, a system easier to host, or a piece more
reusable?"* If none, out.

## 2. Non-goals

- Not a production SaaS; no real users or uptime promises.
- Not a benchmark project — numbers matter only as experiment signals.
- Not a general-purpose PaaS: guest systems onboard deliberately against a
  documented contract (§6.1), not arbitrary workloads.
- No *standing* cloud spend: AWS environments are create/destroy per practice
  session with layered cost guardrails (ADR-006.4); idle cost ≈ $0.

## 3. Users & primary use cases

Single user (the owner), four hats: **operator-in-training** (runs guided
experiments), **platform builder** (evolves the lab — this *is* the
practice), **system owner** (brings real systems to rehearse their production
deployments), **future project owner** (extracts hardened pieces).

## 4. Current state (v2 shipped)

- Consolidated architecture runs end-to-end via root `docker-compose.yml` +
  `Makefile` (13 containers incl. Prometheus/Grafana); `make verify` green;
  contracts pinned and live-verified.
- **[EXPERIMENTS.md](../EXPERIMENTS.md):** 16 guided drills (12 compose +
  EXP-20..23 cluster) with calibrated expectations; the catalog is the
  regression suite and keeps catching real defects (v1.1 exit run: 2;
  v2 exit runs: 3 more + 2 design findings). Tags `lab-v1.1`, `lab-v2.0`.
- **Cluster lab (v2):** `make cluster-up` runs the entire stack on kind —
  kustomize base/overlays ([deploy/k8s/](../deploy/k8s/README.md)), local
  registry :5001, ingress-nginx + cert-manager lab CA
  (api/auth.lab.local), zero-trust network policies, migration Jobs,
  single/multinode profiles, `make drift-check` (ADR-002.4) and CI phase 1
  (ADR-010.2). Observability stays compose-side until v3 (`lab-obs`
  reserved).
- era-1 mined and archived: `legacy_project/` → branch `archive/era-1`.

## 5. The experiments framework

Experiments are the lab's product *and* its test suite:

- **Now:** `EXPERIMENTS.md` — human-guided runbooks over `make` targets
  (Goal / Watch / Steps / falsifiable Expect / Validates / Cleanup).
- **v4:** definitions move to YAML-with-prose and gain machine-checked
  assertions (`make experiment E=<id>` → pass/fail); CI runs a smoke subset
  (ADR-004.1/.2).
- **v6:** Mission Control renders the library and records outcomes.
- **Always:** findings worth keeping get write-ups in
  [experiments/](experiments/); design flaws feed the
  [conceptual review](review/CONCEPTUAL_REVIEW.md) and new ADRs.

## 6. Host platform & guest systems

### 6.1 Onboarding contract
Draft rules in the PRD era; formalized as `HOST_CONTRACT.md` in v3 and proven
by a trivial **hello-guest fixture** before real tenants (ADR-001.4).
Essentials: containers for every component, health + Prometheus metrics, a
launch definition (components, shared-infra needs, port block, secrets), at
least one guided experiment, and a real-deployment plan. Isolation:
namespace/compose-project per guest + documented port ranges; shared lab
infra by default with per-guest BYO where production fidelity demands it
(ADR-007.1/.2).

### 6.2 mycelium (formerly KnowledgeManager) (`/home/fbarroso/forest/mycelium`)
YouTube → knowledge-graph pipeline (GraphRAG) + StagesUI front-end (GraphDash
was retired upstream in phase 4) + systemic-control. Onboards in v7: **pipeline
first**, behind a mandatory deterministic fake-LLM mode (greenfield — unbuilt
upstream today); **adopts the lab's queue conventions** (the migration is the
exercise); real-key runs are explicit budgeted experiments (ADR-007.3/.4).

### 6.3 agentic-workflow-manager (`/home/fbarroso/forest/loam`)
TS monorepo launching Claude Code agents in per-run sandboxes + a read-only
knowledge UI. Onboards in v7: agent runs become **Kubernetes Jobs** via a
loam-side runner adapter (the deployment plan's core artifact); tokens via
k8s Secrets → AWS Secrets Manager; experiments assert the operational
envelope (lifecycle, artifacts, resource limits) — not output quality
(ADR-007.5/.6/.7). Mission Control borrows loam's UI patterns but stays
independent while both projects are in motion (ADR-005.5).

## 7. Mission Control (v6)

One place to control and see the whole lab. Built (Next.js/React) rather than
assembled; a thin **lab-controld** daemon exposes REST/SSE (ADR-005.6) but
executes the same `make` targets everything else uses — one source of truth. Capability
ladder: visibility → control (launch systems/targets) → experiment library →
multi-environment (compose/kind/AWS). A read-only status page ships early, in
v3 (ADR-001.3, ADR-005).

## 8. Roadmap

Resequenced 2026-07-10 (ADR-001.2): **AWS before Mission Control**. Each
phase has a self-contained brief in [phases/](phases/) — work breakdown, exit
experiments, acceptance. Experiment IDs: EXP-<phase>x.

| Phase | One line | Exit |
|---|---|---|
| ✅ v1 — Foundation | Consolidated stack runs, verified, monitored, simulated | shipped |
| ✅ v1.1 — Guided experiments | EXPERIMENTS.md catalog + enablers | `lab-v1.1` (2026-07-10) |
| ✅ v2 — Cluster lab | Entire stack on kind (kustomize, registry, netpols, ingress+TLS); CI phase 1; legacy mined→archived | `lab-v2.0` (2026-07-10) |
| v3 — Observability | kube-prometheus-stack, Tempo traces end-to-end, OpenSearch logs, ntfy alerts, SLO baselines; **+ thin status page + hello-guest/contract** | EXP-30..34 → `lab-v3.0` |
| v4 — Hardening & assertions | definitions.json topology, retry queues, idempotency, outbox+results (status lifecycle!), JWKS, sessions, roles; experiments → YAML+scored; Chaos Mesh; Go loadgen; CI phase 2 | EXP-40..45 → `lab-v4.0` |
| v5 — AWS track | Terraform+EKS sessions: RDS+S3 managed, Route53/ACM, budget+reaper guardrails, OIDC pipeline; idle ≈ $0 | EXP-50..55 → `lab-v5.0` |
| v6 — Mission Control | Next.js UI + lab-controld over make; systems model; experiment library; terminal-free sessions | EXP-60..63 → `lab-v6.0` |
| v7 — Guest systems | KM pipeline (fake mode, lab conventions) + loam agent Jobs; deployment plans written & rehearsed; one guest on AWS | EXP-70..74 → `lab-v7.0` |
| v8 — Extraction | templates/ (auth, worker, publisher) + pattern docs + the one Helm chart; bootstrap-in-a-day proven | EXP-80..82 → `lab-v8.0` |

Parked / rejected (see ADRs): ECS track (rejected — EKS continuity), service
mesh/mTLS (rejected — cert-manager at ingress only), Spot nodes (optional
later), agent cost-ceiling assertions (until loam exposes usage), deeper
loam↔Mission-Control integration (revisit when both stabilize), Redis-backed
lockout (only if auth-service scales >1 replica in drills).

## 9. Success metrics

- **Practice:** each phase's exit experiments executed with write-ups
  (era-1's k6 analyses are the quality bar); time-to-diagnose trending down.
- **Validation:** the experiment catalog passes on every milestone (it is the
  lab's regression suite); phase exits are tagged (`lab-vN`).
- **Hosting:** both real guest systems launch observable from Mission
  Control; their real-deployment plans exist and were rehearsed.
- **Cloud:** AWS sessions routine — deploy, drill, destroy; idle $0; session
  cost known and recorded per write-up.
- **Reuse:** ≥1 real project bootstrapped from the templates in <1 day
  (EXP-80).

## 10. Decisions & remaining openness

All 2026-07-10 open questions were answered by the owner and recorded as
**[ADR-001..010](decisions/)**; phase briefs embed them where implemented.
New questions get new ADR entries (supersession by reference — see
decisions/README).

Deliberately still open: nothing blocking. Deferred-by-choice items are
listed under "Parked / rejected" in §8 and inside the relevant ADRs.

## 11. v1.1 acceptance (current release)

- [x] **Validation run (2026-07-10):** EXPERIMENTS.md top to bottom; all 12
      pass — EXP-01/EXP-09 after mid-run fixes (RabbitMQ healthcheck,
      publish.py persistence), EXP-12 discovery documented. See
      [experiments/2026-07-10-v1.1-catalog-run.md](experiments/2026-07-10-v1.1-catalog-run.md).
      Tagged `lab-v1.1`.
- [x] Catalog + enablers shipped and live-verified during development.
- [x] PRD reflects AWS track, Mission Control, host contract, guest systems.
- [x] Methodology in place: phases/ briefs + decisions/ ADRs (2026-07-10).
