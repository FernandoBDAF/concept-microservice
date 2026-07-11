# ADR-007 — Host contract & guest systems (2026-07-10)

## 007.1 Guest infra: per-guest choice, shared by default
**Context:** shared infra = fewer containers + inherited observability; BYO =
production-faithful isolation. **Decision:** the onboarding contract asks each
guest to declare needs; default is the lab's shared instances (separate
DBs/vhosts/buckets per guest); a guest whose production plan is standalone may
BYO to rehearse it. **Consequences:** resource-lean by default, honest when
it matters.

## 007.2 Isolation: namespace-per-guest + documented port ranges
**Decision:** k8s — namespace per guest, default-deny network policies,
ingress by hostname. Compose — project-per-guest, each allocated a documented
host-port block (e.g. 41xx hello-guest, 42xx KM, 43xx loam). Written into the
contract. **Consequences:** no collisions with lab ports; guests invisible to
each other unless explicitly allowed.

## 007.3 KnowledgeManager: pipeline first, fake-LLM mode required
**Context:** KM = GraphRAG pipeline + GraphDash/StagesUI + systemic-control;
pipeline drills could burn real API credits. **Decision:** onboard the
ingestion pipeline first, with a mandatory deterministic fake-LLM mode as a
contract requirement; real-key runs are explicit budgeted experiments; UIs
onboard after the pipeline is observable. **Consequences:** burst/backlog
drills with realistic shape and zero spend; the fake mode is KM-side work to
plan for.

## 007.4 KM adopts the lab's queue conventions
**Context:** the reusable-template claim needs a real migration test.
**Decision:** during onboarding, KM's pipeline stages port onto the lab
envelope + exchange/queue/DLQ (+retry, ADR-008.1) conventions — the rework is
the exercise. **Consequences:** one observability story; first real evidence
the conventions transfer; onboarding takes longer than wrap-as-is.

## 007.5 loam agent sandboxes = Kubernetes Jobs
**Context:** loam launches Claude Code agents in Docker containers;
"containers launching containers" on k8s needs a mechanism; a production
agent farm on EKS would use Jobs. **Decision:** an agent run = a k8s Job with
resource limits, TTL, log/artifact capture; a loam-side runner adapter is the
core deliverable of its deployment plan. Docker-socket/DinD rejected for
shared clusters. **Consequences:** quota-able, observable agent runs; the
adapter is real integration work with the loam repo.

## 007.6 Agent credentials: k8s Secrets now → Secrets Manager on AWS
**Decision:** lab mode — tokens (CLAUDE_CODE_OAUTH_TOKEN, gh) as k8s Secrets
created by make from local .env, never committed; AWS mode — same manifests
fed by Secrets Manager via external-secrets/CSI. Matches ADR-009.3.
**Consequences:** one injection pattern, two backends; rotation practicable.

## 007.7 Agent-run experiments assert the operational envelope
**Decision:** assertions = lifecycle & artifacts (completion within wall-clock
budget; logs + branch/diff captured; sandbox cleaned) and resource envelope
(limits enforced; hung/OOM agents killed via a deliberate drill). Cost
ceilings deferred until loam exposes usage metrics; output-quality gates are
loam's concern, not the lab's. **Consequences:** drills stay meaningful
despite nondeterministic agents.
