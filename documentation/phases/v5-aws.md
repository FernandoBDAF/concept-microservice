# Phase v5 — AWS track

**Status:** code-complete (execution pass 2026-07-19: stacks, overlay,
guardrails, pipeline implemented + statically verified; **never applied**
— step-0 account + EXP-50..55 remain) — state ledger in
[v5-HANDOFF.md](v5-HANDOFF.md) · **Depends on:** v4 (deploys the hardened
lab) ·
**Exit tag:** `lab-v5.0` · **Decisions in force:** ADR-006 (all), ADR-009.3
(Secrets Manager layer), ADR-010.2 (CI phase 3)

## Mission

The lab deploys to **real AWS on demand and tears down cleanly**: one command
up, drills, one command down — idle cost ≈ $0, session cost known. This phase
is deliberately *operations* practice: Terraform discipline, EKS, managed
services, cost control, real DNS/TLS, and the OIDC deploy pipeline.

## Context a fresh session needs

- v2's kustomize overlays are the deployment unit — v5 adds an `aws` overlay,
  not a new manifest tree. v4's definitions.json / secrets patterns carry over.
- Everything below happens in a **dedicated AWS account, one pinned region**
  (ADR-006.5) — creating that account (+ SSO/IAM user, least-privilege role
  for tooling) is step 0 and a manual prerequisite.
- Cost posture (ADR-006.4): AWS Budgets alarm (suggest $50/mo, alert
  50/80/100% → ntfy/email), session lifecycle via make, TTL-tag reaper.
  Nothing persists between sessions except: tfstate (S3+DynamoDB lock), ECR
  images, Route53 zone, the budget itself.

## Work breakdown

1. **Terraform skeleton (ADR-006.1):** `deploy/aws/` — remote state
   (S3+DynamoDB), modules/envs split; `make aws-plan/aws-up/aws-down`
   wrapping plan/apply/destroy with explicit confirmation.
2. **Persistent base stack (near-$0):** ECR repos (7 images), Route53 zone
   for the lab domain (ADR-006.6), budget+alarms, the reaper (EventBridge →
   Lambda deleting `ttl`-tagged expired resources; aws-nuke rejected in favor
   of scoped reaper since the account is dedicated — revisit if cruft wins).
3. **Session stack:** VPC (community module), EKS (managed node group,
   2–3 smallish nodes), RDS Postgres (both DBs; parameter group; backups on)
  , S3 bucket replacing MinIO, ACM cert + ALB ingress controller;
   self-hosted in-cluster: redis, rabbitmq (+definitions ConfigMap), mongo
   (ADR-006.3).
4. **Kustomize `aws` overlay:** images from ECR; MinIO env → S3 endpoints/
   IRSA role for api-service (code already speaks S3 API — verify presigned
   URL behavior on real S3); DSNs from Secrets Manager via external-secrets
   (ADR-009.3); ingress hosts `api.<domain>`, `grafana.<domain>` with ACM.
5. **Observability on EKS:** kube-prometheus-stack via the same values,
   Tempo; OpenSearch is optional per session (RAM/cost) — decide per drill;
   ntfy keeps working from anywhere.
6. **Image pipeline (ADR-006.7):** first sessions push from laptop
   (`make images REGISTRY=<ecr>`); then GitHub Actions OIDC role →
   build+push to ECR → `kubectl apply -k` deploy job (manual trigger +
   on-tag), keeping make as fallback. CI phase 3 (ADR-010.2).
7. **Session runbook:** `documentation/deployment/AWS_SESSION.md` — start
   (aws-up ~20 min, checkpoints), verify (smoke experiments), cost check
   (Cost Explorer tags query), end (aws-down + verify $0 residuals). Every
   session logs its actual cost in the experiment write-up.

## Out of scope

Mission Control AWS target (v6 — sessions are make-driven here), guests on
AWS (v7), multi-region/HA-for-real, Spot nodes (deferred — optional later,
ADR-006.4), ECS track (rejected, ADR-006.2).

## Exit experiments

- **EXP-50 — Session lifecycle:** `make aws-up` → EXP-02 smoke + EXP-04 burst
  pass against `api.<domain>` → `make aws-down` → assert zero session
  resources remain (script over tagged resources) and session cost ≤ budget
  (record actual $ in write-up).
- **EXP-51 — Catalog on EKS:** the scored smoke subset + EXP-06 outage +
  EXP-21 node kill (real node!) pass on EKS; note behavioral diffs vs kind
  (LB warmup, EBS-backed PVs, AZ awareness).
- **EXP-52 — AZ/node failure:** terminate a node (or AZ-constrained group)
  under load; PDBs + rescheduling hold; alert fires to ntfy; write-up.
- **EXP-53 — RDS failover drill:** trigger RDS reboot-with-failover under
  load; measure connection-pool recovery vs SLO; document pool settings
  changes if needed.
- **EXP-54 — Pipeline deploy:** a commit-tagged change reaches EKS through
  Actions OIDC→ECR→apply with no laptop credentials; rollback via previous
  image tag.
- **EXP-55 — Reaper proof:** create a decoy tagged resource with short TTL;
  reaper kills it; budget alarm test-fires once (then thresholds restored).

## Acceptance

- [ ] EXP-50..55 pass, each with a cost line in its write-up
- [ ] Idle month = $0 ± cents (base stack only); documented in AWS_SESSION.md
- [ ] aws-up/aws-down are the only workflow; no click-ops resources exist
- [ ] Tag `lab-v5.0`
