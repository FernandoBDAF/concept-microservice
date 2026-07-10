# Inferred Intent & Feature Reconstruction

**Date:** 2026-07-10 · **Sources:** git history (`1efec36` old tree, commit
messages), `legacy_project/`, planning documents in
[documentation/planning/](../planning/), and the current codebase.

This reconstructs what the project set out to do — including things attempted
and later archived — so the [PRD](../PRD.md) can build on intentions, not just
surviving code.

## Stated purpose (owner's framing)

A concept project built while studying Kubernetes and planning how to deploy
future work. Two concrete goals:

1. **Generic, reusable infrastructure implementations** — queue, cache,
   authentication (and later storage/object-storage) built abstractly enough
   to lift into real projects.
2. **An operations practice environment** — control the cluster through a
   Makefile, watch it through a UI, and run fake work (load tests, floods,
   failures) to rehearse realistic operational scenarios.

## Era 1 — the microservices k8s lab (up to `1efec36`, early 2026)

Evidence: old tree `k8s/` + `services/` (now `legacy_project/`), guides, k6 jobs.

What existed and clearly worked at least partially:

- **Six services** behind HTTP: auth (Node/TS), profile (Go, orchestrator),
  cache (Go+Redis), storage (Go+Postgres), queue (Go+RabbitMQ), worker (Go) —
  i.e., each generic infrastructure concern deliberately isolated as its own
  service ("in a generic way so I could later use for real cases").
- **A real kind-based cluster lab**: `kind-config.yaml` + `kind-multinode.yaml`,
  ingress-nginx, metrics-server, storage classes, **default-deny network
  policies**, per-service manifests numbered 01–06 with resource
  requests/limits, probes, security contexts, and heavy educational annotation.
- **Operational scripting** (the "control with Makefile" intent, then in bash):
  `setup-cluster.sh`, `setup-cluster-enhanced.sh`, `build-all-images.sh`,
  `load-images-to-kind.sh`, `monitor-health.sh`, validation scripts.
- **Load testing as a first-class activity**: a `k8s/k6/` suite with
  validation/load/stress jobs for the profile API, configmaps/PVCs for test
  data, a tracker, and **written analyses of test runs**
  (`k6/analyses/profile-crud-load-test-analysis.md`) — the "practice on
  diverse simulations" goal, partially achieved.
- **Debug/validation harnesses** for the queue path
  (`debug/validate_rabbit_worker_queue/`, `reestruct_queue_flow/` with Go
  publisher/consumer pairs) — evidence of hands-on failure investigation.
- **Guides as curriculum**: FOUNDATIONS → SERVICES → INTEGRATION → ADVANCED →
  OBSERVABILITY deployment guides + HELM-IMPLEMENTATION + IMPLEMENTATION_HISTORY;
  process docs (TODO, TRACKER&MANAGER, TASK_EXECUTION, CURSOR/LLM instructions
  for AI-assisted development).

Inferred intents not fully reached in era 1: Helm packaging (guide written,
no charts survive), the observability stack itself (guide exists; no
Prometheus/Grafana manifests survive at top level), and multi-node operation
(config present; commit messages validate only service paths).

## Era 2 — the consolidation (Jan 2026 working tree → `4ac1166`)

Evidence: `CONSOLIDATED_SERVICE_PLAN.md`, `MASTER_IMPLEMENTATION_PLAN.md`,
`PLAN_AUTH_SERVICE_AND_DEPLOYMENT.md`, `BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md`,
`GRAPHRAG_AND_CONCURRENCY_PLAN.md`, and the code.

The pivot: collapse cache/storage/queue services into one Go api-service with
direct infrastructure clients, keep auth separate (rewritten to modern TS),
and move the interesting distributed behavior to the *async* side — four task
pipelines with typed contracts (`shared/contracts/`), three Go operational
workers, and an ambitious Python GraphRAG worker (documents → MinIO →
knowledge graph in MongoDB, LLM entity extraction) as the "real-ish" workload.

Intents visible in era 2:

- **Contract-first messaging** (MESSAGE_FORMAT.md, ROUTING_KEYS.md) — the
  reusable-queue goal shifted from a queue *service* to queue *conventions*.
- **A meaningful async workload** (GraphRAG) to make load/failure practice
  non-trivial (long processing, external APIs, multi-store writes).
- **Document pipeline** (MinIO + upload/status/download endpoints) as the
  generic object-storage piece.
- **Auth as a reusable standalone** (JWT access+refresh, lockout, audit log,
  rate limits, zod-validated config) — the most template-ready piece.
- Deployment intent survived as *plans* (`deployment/CLUSTER_VISION.md`
  describing the target `kubectl get pods` end-state, PLAN_AUTH_SERVICE_AND
  _DEPLOYMENT.md) but era 2 never rebuilt the cluster substrate; work stopped
  at per-service manifests.

What era 2 lost (unintentionally, by archiving rather than adapting): the kind
cluster, network policies, k6 suite, operational scripts, and observability
curriculum — i.e., most of goal 2.

## Era 3 — refactor + v1 (2026-07)

The [2026-07 refactor](../refactor/2026-07-full-refactor.md) made era 2's
architecture actually run (builds, tests, contracts reconciled, live E2E), and
v1 (this change) restores the first slice of goal 2: a monitoring UI
(Prometheus + Grafana), worker metrics actually exposed, and a Makefile-driven
simulation harness (load, burst, poison, outage drills) against the compose
stack. The k8s lab restoration is deliberately deferred to PRD v2 so it can be
rebuilt around the consolidated architecture instead of resurrected wholesale.

## Feature inventory vs. intent (scorecard)

| Intended capability | Era 1 | Era 2 | v1 (now) | PRD target |
|---|---|---|---|---|
| Generic auth service | ✅ | ✅ rewritten, tested | ✅ | v5 template (RS256/JWKS) |
| Generic queue infra | ✅ service | ✅ contracts+workers | ✅ verified E2E | v5 template (retry/idempotency/outbox) |
| Generic cache infra | ✅ service | ➖ folded into api | ✅ working | v5 pattern write-up |
| Object storage pipeline | ✖ | 🟡 built, unverified | ✅ verified | v4 scenarios (orphans, big files) |
| Kubernetes cluster lab | ✅ kind+policies+ingress | ✖ archived | ✖ (manifests only) | **v2 core** |
| Makefile as control plane | 🟡 bash scripts | ✖ | ✅ compose-scoped | v2 extends to cluster |
| Monitoring UI | 🟡 guide only | 🟡 metrics unshipped | ✅ Prometheus+Grafana | v3 dashboards/alerts/traces |
| Load & failure simulations | ✅ k6 suite + analyses | ✖ | ✅ first drills | **v4 scenario library** |
| Real-ish workload (GraphRAG) | ✖ | 🟡 stub-mode | 🟡 stub-mode verified | open question |
