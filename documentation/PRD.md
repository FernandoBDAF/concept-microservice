# PRD — Microservices Operations Lab

**Status:** living document, v1.1 shipping · **Owner:** Fernando ·
**Last updated:** 2026-07-10

## 1. Vision

A personal **operations practice platform**: a small but honest distributed
system (API, auth, queues, cache, object storage, async workers) that can be
stood up, broken, observed, and repaired on demand — locally first, then on
**real AWS infrastructure** — so that operating Kubernetes, cloud deployments,
and distributed infrastructure becomes muscle memory before it's needed on
real projects.

The lab is not only about its own built-in services. It is a **host platform**
able to incorporate other systems: a centralized **Mission Control UI** will
choose which system/stack to launch, drive **guided experiments** against it,
and give visibility over everything running. Two real guest systems are
already slated (§6): a knowledge-manager (GraphRAG expansion) and an
agentic-workflow-manager — the goal is to plan their *real* deployments and
rehearse them here first.

Its second output remains **generic, reusable infrastructure pieces** (auth
service, queue conventions + worker template, cache pattern, storage
pipeline) that future real projects lift wholesale.

**One-line test for scope decisions:** *"Does this make a realistic
operational scenario practicable, a system easier to host, or a piece more
reusable?"* If none, out.

## 2. Non-goals

- Not a production SaaS; no real users or uptime promises.
- Not a benchmark project — numbers matter only as experiment signals.
- Not a general-purpose PaaS: guest systems are onboarded deliberately, one
  by one, against a documented contract (§6.1) — not arbitrary workloads.
- No *standing* cloud spend: AWS environments must be create/destroy per
  practice session with cost guardrails (§OQ-AWS); idle cost ≈ $0.

## 3. Users & primary use cases

Single user (the owner), four hats:

1. **Operator-in-training** — runs guided experiments (EXPERIMENTS.md, later
   the UI library): floods, outages, scaling, rollouts, cloud deploys;
   practices diagnosing from dashboards/logs/queues, not from code.
2. **Platform builder** — evolves the lab itself (cluster, observability,
   Mission Control, AWS pipeline) — this *is* the practice.
3. **System owner** — brings real systems (knowledge-manager, agentic
   workflow-manager) to plan and rehearse their production deployments.
4. **Future project owner** — extracts hardened pieces into real projects.

## 4. Current state (v1.1)

- Consolidated architecture runs end-to-end via root `docker-compose.yml` +
  `Makefile`: api-service (Go), auth-service (TS), 3 Go workers, Python
  graphrag worker (stub mode), Postgres×2 DBs, Redis, RabbitMQ, MinIO, Mongo.
- All projects build and pass tests (`make verify`); contracts pinned and
  reconciled; live E2E verified (auth → API → queue → workers → DLQs).
- **Monitoring UI:** Prometheus scraping every service + RabbitMQ; Grafana
  with a provisioned Lab Overview dashboard.
- **Simulation harness:** `make sim-smoke / sim-load / sim-burst / sim-poison
  / sim-outage / queues / scale / demo-document`.
- **Guided experiments (v1.1):** root **[EXPERIMENTS.md](../EXPERIMENTS.md)**
  — a numbered catalog (cold start, load, burst, poison, outages, scale-out,
  incident reproduction, TTL expiry, document E2E) where each experiment says
  what to run, what to watch, what to expect, and which implementation claims
  it validates. Experiment write-ups go to `documentation/experiments/`.
  Already proven useful: the harness caught a real incident (hardcoded
  validate rate limit + breaker = full outage — CONCEPTUAL_REVIEW §2).
- Kubernetes: per-service manifests only; era-1 kind lab still archived.

## 5. The experiments framework

Experiments are the lab's product *and* its test suite:

- **Now (v1.1):** `EXPERIMENTS.md` at the repo root — human-guided runbooks
  over `make` targets. Each has: Goal, Watch (specific panels/queries),
  Steps, Expect, Validates, Cleanup. Running the full catalog *is* the
  acceptance test of the implementation.
- **Later (Mission Control, §v5):** the file becomes an **experiment library
  in the UI** — browsable sessions that launch the load/failure, embed the
  relevant dashboards, and (v4) assert expectations automatically instead of
  asking the user to eyeball them.
- **Always:** findings worth keeping get a write-up in
  `documentation/experiments/` (the era-1 k6 analyses are the quality bar).
  Experiments that expose design flaws feed CONCEPTUAL_REVIEW and this PRD.

## 6. Host platform & guest systems

### 6.1 Onboarding contract (draft — hardens in v5/v7)

A system becomes launchable in the lab when it provides:

1. **Containers** for every component (compose file and/or k8s manifests),
   parameterized by env vars documented in a CONTRACTS-style file.
2. **Health** (`/health`, `/ready`) and **Prometheus metrics** endpoints.
3. **A launch definition**: which components, which shared infra it needs
   (Postgres? Redis? RabbitMQ? MinIO? Mongo?), ports, secrets required.
4. **At least one guided experiment** (load or failure drill) proving it's
   observable in the lab.
5. **A deployment plan** for its real target (what AWS shape it needs).

Open: whether guest systems share the lab's infra services or bring their own
(§OQ-HOST).

### 6.2 Guest system: knowledge-manager (`~/repo/KnowledgeManager`)

Expansion of this repo's GraphRAG concept into a real product: extracts
knowledge from **YouTube videos** into knowledge graphs and displays them in
a UI. Known shape (from repo recon): a GraphRAG pipeline module, two web
front-ends (`GraphDash`, `StagesUI`), a `systemic-control` module, and
substantial pipeline-observability analysis docs — i.e., it already thinks in
stages/observability terms, which maps well onto the lab's Prometheus/Grafana
stack.

Practice goals: containerize the pipeline + UIs; define its ingestion
workload as experiments (batch video ingestion = a natural burst/backlog
drill with real LLM cost knobs); plan its real deployment (GPU/API-cost
questions included); eventually run its pipeline against the lab's
RabbitMQ/MinIO/Mongo instead of bespoke plumbing.

### 6.3 Guest system: agentic-workflow-manager (`~/repo/Raine/loam`)

TypeScript monorepo ("loam"): an agentic development methodology + knowledge
system whose workflows **launch Claude Code agents in Docker sandboxes**
(per-run container, topic branch, host-side `gh`), plus a local read-only
knowledge UI. Deployment practice here is distinctive: **containers that
launch containers** (Docker socket? DinD? k8s Jobs? — §OQ-AWM), secret
handling for agent OAuth tokens, log/artifact collection per run, and cost
control for agent runs. Its knowledge UI is also a candidate first tenant for
Mission Control integration (both are "local web UI over derived state").

Practice goals: plan the real "agent farm" deployment (queue of agent jobs →
sandboxed runs → artifacts/logs → knowledge updates), simulate agent-run
workloads in the lab (an agent run is just a long-lived job — the lab's
slow-consumer patterns apply), rehearse failure modes (hung agent, sandbox
OOM, token expiry).

## 7. Mission Control (the centralized UI)

One place to **control** and **see** the whole lab. Replaces nothing at
first — it wraps what `make` already does (§OQ-UI2: it must not become a
second source of truth; it drives the same compose/k8s/scripts).

Capability ladder (each rung is independently useful):

1. **Visibility:** what's running (systems, containers/pods, health), links
   to Grafana/Prometheus/RabbitMQ/MinIO consoles, live status.
2. **Control:** launch/stop a *system* (the lab stack, a guest system, an
   infra-only profile), scale components, trigger migrations/resets.
3. **Experiment library:** browse the EXPERIMENTS catalog, run one
   (streaming output), embed its "Watch" dashboards next to it, record
   outcomes to `documentation/experiments/`.
4. **Environments:** local-compose vs kind vs AWS as selectable targets for
   the same actions (this is where AWS practice and the UI meet).

## 8. Roadmap

Milestones are outcome-defined; each ends with experiments that prove it.
Numbering is the default order; §OQ-SEQ questions the two marked ⇅.

### v1.1 — Guided experiments (this change)
Root EXPERIMENTS.md catalog over the compose stack; small enablers
(env-tunable rate limits for incident reproduction, per-message TTL flag,
document-E2E demo, consumer scaling target); `documentation/experiments/`
as the write-up home. **Exit:** owner runs the catalog top to bottom and
every Expect holds (this validates v1 itself).

### v2 — Restore the cluster lab (kind)
`make cluster-up` runs the entire v1 stack on kind; every v1 experiment works
against the cluster; namespaces, probes, limits, PDBs, default-deny network
policies, ingress; config layering decided (§OQ-P1) so compose/k8s stop
drifting; in-cluster Prometheus/Grafana incl. node/kubelet metrics.
**Exit experiment:** kill a node in multinode kind mid-load, watch
rescheduling, write the analysis.

### v3 — Observability depth
OpenTelemetry tracing end-to-end (HTTP → AMQP headers → workers; the envelope
`trace_id` becomes real), log aggregation, per-service dashboards, alert
rules (queue depth, DLQ growth, breaker open, p99), SLOs per service.
**Exit:** every catalog experiment diagnosable from UI alone.

### v4 — Experiment library v2 (assertions)
Experiments gain machine-checked expectations (thresholds/queries asserted,
not eyeballed); catalog expands (cache stampede, DB failover, rolling deploy
under load, HPA/KEDA scale-out, slow-consumer timeouts, chaos tool §OQ-S1).
**Exit:** `make experiment E=<id>` returns pass/fail; CI can run a subset.

### v5 ⇅ — Mission Control UI
The §7 ladder, rungs 1–3, over local targets (compose + kind). The
EXPERIMENTS.md file is superseded by the UI library (file stays as the
canonical experiment *definitions* the UI reads — format decision §OQ-UI3).
**Exit:** run an entire practice session (launch → experiment → diagnose →
record) without touching a terminal.

### v6 ⇅ — AWS deployment track
The lab deploys to real AWS on demand and tears down cleanly: IaC
(§OQ-AWS1), EKS-vs-ECS decision (§OQ-AWS2), ECR image pipeline, managed-vs-
self-hosted infra choices per service (RDS? ElastiCache? Amazon MQ? S3
replaces MinIO?), secrets (SSM/Secrets Manager), cost guardrails + budget
alarms + one-command destroy (§OQ-AWS4). Mission Control gains the AWS
target (rung 4). **Exit experiments:** deploy → run the smoke/burst catalog
against AWS → induce a node/AZ failure → destroy; total session cost within
budget and idle cost $0.

### v7 — Guest systems onboarding
knowledge-manager and agentic-workflow-manager onboarded per §6.1:
containerized launch definitions, health+metrics, their own experiment
chapters, and written **real-deployment plans** (the original point). The
host contract hardens from draft to documented standard.
**Exit:** each guest system launchable from Mission Control locally, at
least one system also deployed to AWS in a practice session.

### v8 — Extraction & reuse
Hardened pieces become copy-ready templates (auth w/ RS256+JWKS, worker w/
real retry+idempotency+outbox, cache pattern, storage pipeline), each proven
by the experiments that beat on it; CI story (§OQ-C1) ships with them.
**Exit:** a real project bootstrapped from templates in < 1 day.

### Parked / opportunistic
GraphRAG real pipeline in-lab (subsumed by knowledge-manager onboarding —
keep the stub as the lab's slow-consumer); multi-env story (staging
namespace, blue/green) — inside v6/v7; repo split for templates (§OQ-R1).

## 9. Success metrics

- **Practice:** each milestone's exit experiment executed with a written
  analysis in `documentation/experiments/`; time-to-diagnose trending down.
- **Validation:** the experiment catalog passes end-to-end on every milestone
  (it is the regression suite for the lab itself).
- **Hosting:** two real guest systems launch, observable, from Mission
  Control; their real-deployment plans exist and were rehearsed.
- **Cloud:** AWS practice sessions are routine: deploy, drill, destroy,
  idle cost $0, session cost known in advance.
- **Reuse:** ≥1 real project bootstrapped from the templates in < 1 day.

## 10. Open questions

Grouped; each blocks or shapes the milestone in parentheses. Decisions get
recorded in `documentation/decisions/` as lightweight ADRs (start at v2).

### Sequencing (roadmap)
- **OQ-SEQ1:** Mission Control (v5) before AWS (v6), as ordered — or is a
  minimal UI pointless until there are two targets, arguing AWS first?
- **OQ-SEQ2:** could Mission Control rung 1 (visibility only) ship much
  earlier (alongside v3) as a thin status page, deferring control rungs?
- **OQ-SEQ3:** do guest systems really wait for v7, or does onboarding
  knowledge-manager early (compose-only, no UI) pressure-test the host
  contract while the lab is still simple?

### Platform (v2)
- **OQ-P1:** kustomize overlays or Helm charts? (Helm was era-1's aspiration;
  kustomize is less magic for a learning lab.)
- **OQ-P2:** kind vs k3d vs minikube; is multinode-on-laptop worth the RAM?
- **OQ-P3:** image path into the cluster: kind-load, local registry, or
  ephemeral registry?
- **OQ-P5:** does compose remain first-class after v2, or demote to "quick
  smoke only" to stop double-maintaining configs?

### AWS (v6)
- **OQ-AWS1:** IaC tool — Terraform (industry default), OpenTofu, CDK, or
  Pulumi? (This choice is itself practice; pick what future real work uses.)
- **OQ-AWS2:** EKS (max k8s continuity with v2, higher cost/complexity) vs
  ECS/Fargate (cheaper, AWS-idiomatic, less k8s practice) — or both, as
  separate practice tracks?
- **OQ-AWS3:** managed swaps — RDS for Postgres, ElastiCache for Redis,
  Amazon MQ for RabbitMQ, S3 for MinIO, DocumentDB/Atlas for Mongo: which
  swaps are *the practice* vs which stay self-hosted for cost/fidelity?
- **OQ-AWS4:** budget: monthly cap? per-session cap? Billing alarms +
  auto-teardown (e.g., TTL-tagged resources + scheduled destroy)?
- **OQ-AWS5:** account hygiene — dedicated AWS account/OU for the lab?
  SSO/root handling, region pinning?
- **OQ-AWS6:** DNS/TLS for real endpoints (Route53 + ACM) or keep everything
  behind port-forwards/SSM tunnels?
- **OQ-AWS7:** what CI/CD shape deploys to AWS — GitHub Actions OIDC → ECR →
  cluster, or manual `make aws-*` first?

### Mission Control (v5)
- **OQ-UI1:** stack — build (Next.js/React? Go + HTMX?) vs assemble
  (Backstage? Portainer + Grafana links?) — how much of the UI's value is
  building it ourselves (practice) vs having it (utility)?
- **OQ-UI2:** control mechanism — UI shells out to the same `make` targets
  (single source of truth) vs a proper API daemon that both `make` and UI
  call? Where does state live?
- **OQ-UI3:** experiment definition format the UI reads — keep markdown with
  a strict structure, or move definitions to YAML with markdown prose?
- **OQ-UI4:** auth for the UI itself (localhost-only forever, or does AWS
  mode force real auth)?
- **OQ-UI5:** does loam's read-only knowledge UI (§6.3) inform or even embed
  into Mission Control (both render derived state over local artifacts)?

### Host contract & guest systems (v7)
- **OQ-HOST1:** shared infra (guests use the lab's Postgres/RabbitMQ/MinIO)
  vs bring-your-own (full isolation, more RAM)? Per-guest choice?
- **OQ-HOST2:** namespace/compose-project isolation model per guest; port
  allocation policy?
- **OQ-KM1:** knowledge-manager's LLM spend during experiments — fake mode
  requirement before onboarding? Which of its modules onboard first
  (pipeline vs UIs)?
- **OQ-KM2:** does knowledge-manager adopt the lab's queue contracts
  (envelope/DLQ conventions) or keep its own pipeline plumbing?
- **OQ-AWM1:** loam agents-in-containers on the lab: Docker socket mount
  (simple, insecure), DinD, or k8s Jobs as the sandbox (maps to real
  deployment)? Resource caps per agent run?
- **OQ-AWM2:** agent OAuth tokens/secrets handling in lab and AWS modes?
- **OQ-AWM3:** what does an "agent run" experiment assert — completion,
  artifact presence, cost ceiling, wall-clock budget?

### Messaging / auth / observability (carried from v1 review — unchanged)
- **OQ-M1..M6:** retry model, idempotency store, outbox, topology ownership,
  TTL semantics, generic-endpoint whitelist (see CONCEPTUAL_REVIEW §3–§5,
  §12).
- **OQ-A1..A7:** RS256+JWKS local verify, revocation, salt→pepper, distributed
  rate limits, secrets tooling, TLS/mTLS scope, role enforcement.
- **OQ-O1..O6:** prometheus-operator vs hand-rolled, Tempo vs Jaeger, Loki vs
  ELK, alert routing, exporter scope, SLO definitions.
- **OQ-S1:** chaos tooling (chaos-mesh? manual scripts?) (v4).
- **OQ-W2/W3:** AMQP-native load gen; interactive vs scored drills (v4).
- **OQ-C1/R1..R4:** CI provider/triggers, template repo split, legacy_project
  archival, ADR process, milestone tagging.

## 11. v1.1 acceptance (this release)

- [ ] **Owner-run validation (next session):** run EXPERIMENTS.md top to
      bottom; every Expect holds or the deviation is written up in
      `documentation/experiments/`.
- [x] EXPERIMENTS.md covers: baseline, smoke, steady load, burst+drain,
      poison/DLQ, worker outage, scale-out, incident reproduction (rate
      limit → breaker), broker outage, TTL expiry, document E2E, cache
      outage discovery.
- [x] Enablers shipped: `.env`-tunable auth rate limits; `publish.py
      --expiration-ms`; `make scale S=<svc> N=<n>`; `make demo-document`
      (first live exercise of the upload→MinIO→publish→consume path);
      `documentation/experiments/` established.
- [x] PRD reflects: AWS practice track, Mission Control UI, host-platform
      contract, both guest systems (with repo recon), experiments framework.
