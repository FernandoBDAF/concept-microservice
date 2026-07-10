# PRD — Microservices Operations Lab

**Status:** living document, v1 shipped · **Owner:** Fernando ·
**Last updated:** 2026-07-10

## 1. Vision

A personal **operations practice lab**: a small but honest distributed system
(API, auth, queues, cache, object storage, async workers) that can be stood
up, broken, observed, and repaired on demand — **controlled through `make`
targets and observed through a UI** — so that operating Kubernetes and
distributed infrastructure becomes muscle memory before it's needed on real
projects. Its second output is a set of **generic, reusable infrastructure
pieces** (auth service, queue conventions + worker template, cache pattern,
storage pipeline) that future real projects can lift wholesale.

**One-line test for scope decisions:** *"Does this make a realistic
operational scenario practicable, or a piece more reusable?"* If neither, out.

## 2. Non-goals

- Not a production SaaS; no real users, uptime promises, or real emails/images.
- Not a benchmark project — performance numbers matter only as scenario
  signals (e.g., "watch p99 degrade under X").
- Not a framework — reusable pieces are *templates to copy*, not libraries to
  version and support (revisit in v5).
- No cloud spend by default; everything must run on one dev machine. Cloud is
  an explicit open question (OQ-P4).

## 3. Users & primary use cases

Single user (the owner), three hats:

1. **Operator-in-training** — runs scenario drills: floods, outages, scaling,
   rollouts; practices diagnosing from dashboards/logs/queues, not from code.
2. **Platform builder** — evolves the lab itself (cluster, observability,
   CI) — this *is* the k8s practice.
3. **Future project owner** — extracts auth/queue/worker/cache/storage pieces
   into real projects with minimal rework.

## 4. Current state (v1 — shipped 2026-07)

- Consolidated architecture runs end-to-end via root `docker-compose.yml` +
  `Makefile`: api-service (Go), auth-service (TS), 3 Go workers, Python
  graphrag worker (stub mode), Postgres×2 DBs, Redis, RabbitMQ, MinIO, Mongo.
- All projects build and pass tests (`make verify`); contracts pinned
  (CONTRACTS.md, ROUTING_KEYS.md, MESSAGE_FORMAT.md) and reconciled with code;
  live E2E verified (auth → API → queue → workers).
- **Monitoring UI:** Prometheus scraping every service + RabbitMQ; Grafana
  with a provisioned overview dashboard (`make up` → localhost:3001).
- **Simulation harness v0:** `make sim-smoke / sim-load / sim-burst /
  sim-poison / sim-outage / queues` — k6 API load plus queue-level drills
  against the compose stack.
- Kubernetes: per-service manifests only; the era-1 kind lab remains archived.

## 5. Roadmap

Milestones are outcome-defined; each ends with a *drill you can run and a doc
that says what you should have seen*. Rough order, no dates.

### v2 — Restore the cluster lab (the core k8s goal)

**Outcome:** `make cluster-up` builds a kind cluster running the entire v1
stack; every v1 drill works against the cluster; `make cluster-*` targets
mirror the compose ones.

- kind cluster (single + multinode profiles), local image build+load pipeline.
- Namespaces, per-service Deployments/StatefulSets with probes, resource
  limits, PDBs; ingress-nginx for api/auth/grafana/rabbitmq-mgmt.
- Default-deny network policies (port era-1's zero-trust model to the new
  five-service shape).
- Config layering decision implemented (OQ-P1: kustomize vs helm) so compose
  and k8s stop drifting.
- Prometheus/Grafana in-cluster (kube-prometheus-stack? OQ-O1) including
  node/kubelet metrics — the "watch the cluster itself" upgrade.
- Exit drill: kill a node in multinode kind mid-load; watch rescheduling on
  the dashboard; write the analysis like era-1's k6 analyses.

### v3 — Observability depth

**Outcome:** every drill is diagnosable from UI alone (no docker exec).

- Distributed tracing: OpenTelemetry SDKs in all services, context propagated
  HTTP → AMQP headers → workers; backend per OQ-O2 (Tempo vs Jaeger).
  `trace_id` in the envelope becomes real.
- Log aggregation (OQ-O3: Loki vs ELK) with trace-id correlation.
- Per-service Grafana dashboards + RED/USE conventions; alert rules
  (queue depth, DLQ growth, breaker open, p99, worker lag) → where (OQ-O4).
- Exporters: postgres, redis, mongo, minio (OQ-O5 scope).
- SLOs for the lab services (OQ-O6) so drills have pass/fail signals.

### v4 — Scenario library (the practice curriculum)

**Outcome:** a numbered, documented catalog of repeatable drills
(`make drill-<name>` + runbook + expected observations), graduating from v1's
ad-hoc sims. Candidate catalog:

| Drill | Practices |
|---|---|
| Broker outage & recovery | reconnect behavior, backlog drain, TTL losses |
| Poison-message flood | DLQ triage, retry policy (once OQ-M1 decided) |
| Worker crash-loop under load | probes, restart policy, backlog alerts |
| Cache stampede / Redis outage | degradation modes, breaker tuning |
| Postgres failover / disk-full | StatefulSet ops, connection pool behavior |
| Auth-service down | breaker lockout blast radius (validates OQ-A1) |
| Rolling deploy + rollback under load | maxSurge/maxUnavailable, error budgets |
| HPA scale-out on queue depth | KEDA or custom-metrics HPA (OQ-K1) |
| Slow-consumer (graphrag 12h budget) | prefetch, per-message timeouts |
| Chaos injection | tool choice OQ-S1 (chaos-mesh? manual?) |

### v5 — Extraction: the reusable pieces

**Outcome:** a `templates/` (or separate repos, OQ-R1) with copy-ready pieces,
each hardened by the drills that exercised it:

- **Auth template**: RS256/JWKS local verification (from OQ-A1), revocation
  strategy (OQ-A2), role enforcement middleware, pepper decision (OQ-A3),
  distributed rate-limit/lockout (OQ-A4).
- **Worker/queue template**: real retry model (OQ-M1), idempotent consumption
  (OQ-M2), outbox publishing (OQ-M3), topology ownership (OQ-M4), the
  envelope + contract-test harness.
- **Cache pattern**: documented cache-aside with stampede protection and
  invalidation strategy.
- **Storage pipeline**: presigned-URL variant, orphan-reconciliation job.
- CI for all of it (OQ-C1) — the lab's own CI is part of the template value.

### Parked / opportunistic

- GraphRAG real pipeline (needs LLM keys + budget — OQ-W1): keep as the
  "expensive slow consumer" workload, replace with a cheap fake, or drop.
- Multi-env story (staging-like namespace, blue/green) — after v2.
- Real domain beyond profiles/documents — only if a real project adopts the
  templates and feeds back.

## 6. Success metrics

- **Practice:** each vN exit drill executed with a written analysis (era-1's
  k6 analyses are the quality bar); time-to-diagnose in drills trending down.
- **Reuse:** at least one real project bootstrapped from the templates with
  < 1 day of adaptation (the original motivation, made measurable).
- **Honesty:** `make verify` green on main; zero contract drift (CI-enforced
  after OQ-C1); top-level docs describe only what runs.

## 7. Open questions

Grouped; each blocks or shapes the milestone in parentheses. Decisions get
recorded in `documentation/decisions/` as lightweight ADRs (start at v2).

### Platform (v2)
- **OQ-P1:** kustomize overlays or Helm charts for the k8s manifests? (Helm
  was era-1's aspiration; kustomize is less magic for a learning lab.)
- **OQ-P2:** kind vs k3d vs minikube — is kind still the right substrate, and
  is multinode-on-laptop worth the RAM, or should multinode drills be cloud
  bursts (see OQ-P4)?
- **OQ-P3:** image path into the cluster: kind-load (era-1), local registry
  container, or ttl.sh-style ephemeral registry?
- **OQ-P4:** is there any budget/appetite for occasional real-cloud runs
  (spot EKS/GKE) for node-level drills that kind can't simulate honestly?
- **OQ-P5:** does compose remain a supported first-class runtime after v2, or
  demote it to "quick smoke only" to stop double-maintaining configs?

### Messaging (v3/v4/v5)
- **OQ-M1:** retry model — x-death counting, TTL-backoff retry queues, or
  delayed-message-exchange plugin? (Removes the `x-max-retries` fiction.)
- **OQ-M2:** idempotency store — Redis SETNX vs per-service processed-ids
  table? What TTL/retention?
- **OQ-M3:** adopt transactional outbox for api-service publishes? (Fixes the
  MinIO/Postgres/publish consistency gap; also a great drill.)
- **OQ-M4:** topology ownership — RabbitMQ definitions.json at boot, shared
  declaring library, or consumer-owns-queue convention?
- **OQ-M5:** TTL semantics per task type — which tasks may expire, and should
  expirations be routed separately from poison messages?
- **OQ-M6:** does the generic `POST /tasks` endpoint (→ `default-tasks`
  parking lot) stay, get whitelisted, or die?

### Auth & security (v5, informs v2 ingress)
- **OQ-A1:** RS256+JWKS local verification as default, introspection as
  opt-in strict mode — agreed? Key rotation story?
- **OQ-A2:** revocation: token denylist in Redis, short access-token TTL only,
  or stateful refresh sessions table?
- **OQ-A3:** drop the salt column or convert to env-based pepper?
- **OQ-A4:** distributed rate-limiting/lockout (Redis) — needed before
  auth-service scales past one replica.
- **OQ-A5:** secrets in the lab: SOPS, sealed-secrets, or generated-at-setup?
  Where's the line between committed dev defaults and generated secrets?
- **OQ-A6:** TLS inside the lab (ingress-only vs everywhere/mTLS)? Is a
  service mesh (linkerd?) ever in scope, or explicitly out?
- **OQ-A7:** role/authorization model — enforce the existing `role` claim
  anywhere meaningful, or remove it until a real need exists?

### Observability (v3)
- **OQ-O1:** kube-prometheus-stack (operator, heavier) vs hand-rolled
  Prometheus like v1 (lighter, more learning-by-assembly)?
- **OQ-O2:** traces backend: Grafana Tempo (one-vendor stack) vs Jaeger
  (classic)? Sampling policy?
- **OQ-O3:** logs: Loki vs OpenSearch/ELK vs "docker/kubectl logs is enough
  until it isn't"?
- **OQ-O4:** alert routing in a single-operator lab — Grafana alerts to
  where (email? ntfy? nothing but the UI)?
- **OQ-O5:** which infra exporters are worth their resource cost on a laptop?
- **OQ-O6:** define SLOs per service (what's a "good" p99 for the lab)?

### Workloads (v4)
- **OQ-W1:** GraphRAG future — fund real LLM runs occasionally, build a
  deterministic fake pipeline (sleep+fabricated entities) for drills, or
  drop it for a simpler slow-consumer?
- **OQ-W2:** load-gen standard: stay with k6 (era-1 continuity) or add
  AMQP-native load (custom Go publisher) for queue-side drills?
- **OQ-W3:** should drills be interactive (`make drill-x` + human watches) or
  scored/asserted (thresholds fail the run) — or both modes?

### Repo & process (cross-cutting)
- **OQ-C1:** CI — GitHub Actions running `make verify` + contract tests +
  image builds? Runs on which triggers, given a solo repo?
- **OQ-R1:** templates in-repo (`templates/`) vs separate template repos per
  piece? How do improvements flow back from real projects?
- **OQ-R2:** `legacy_project/` (209MB) — keep tracked forever, or move to a
  `legacy` branch / release-archive to slim clones once era-1 material is
  fully mined into docs?
- **OQ-R3:** ADR process — adopt the lightweight decisions/ folder proposed
  above, or keep decisions in PRD edits only?
- **OQ-R4:** versioning/tagging — tag vN milestone completions so drills can
  reference exact lab states?

## 8. v1 acceptance (this release)

- [x] Refactored stack merged to main; `make verify` green.
- [x] `make up` → full stack + Prometheus (9090) + Grafana (3001, provisioned
      overview dashboard) with all scrape targets up.
- [x] Go workers expose `/metrics`; RabbitMQ per-queue metrics enabled.
- [x] `make sim-smoke` / `sim-load` / `sim-burst` (k6), `sim-poison`,
      `sim-outage`, `queues` work against the compose stack and are
      observable in Grafana.
- [x] Documentation centralized under `documentation/` (review, intent, PRD,
      refactor record, deployment, planning archive).
