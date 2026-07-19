# Phase v5 handoff — AWS track

**Audience:** the session that finishes v5. Decisions are settled
(ADR-006); this file sequences the work.

## Execution state (2026-07-19 execution pass — code-complete, never applied)

All build-side steps below (1, 3-9) are **DONE** and statically verified:
three terraform stacks `fmt`+`validate` clean (terraform 1.15.8; modules
resolved live: eks 21.24.0, rds 6.13.1, iam 5.60.0, vpc 6.x; provider aws
6.55.0); aws overlay + kind overlays + obs manifests render; `make verify`
+ `make drift-check` green; reaper has offline unit tests; both workflows
parse. Chart pins verified against the live helm repos: alb-controller
3.4.2, external-secrets 2.8.0 (CRs on `external-secrets.io/v1` — v1beta1
is served:false in that chart), external-dns 1.21.1. Extra beyond this
file's plan: api-service (Go) and graphrag (Python) S3 clients gained an
ambient-credential (IRSA) fallback — the static-key-only clients would
have failed on EKS once the overlay drops the key env.

**Still open (needs the step-0 account / a live session):**

1. Step 0 itself (account, `lab` profile, tfvars) — owner-manual.
2. First applies: bootstrap apply → `make aws-init` → `make aws-base-pack`
   → base apply → NS delegation → ECR seed (steps 1-2 below).
3. GitHub repo vars `AWS_DEPLOY_ROLE_ARN`/`AWS_REGION`; per-session tfvar
   `deploy_role_arn` (grants the CI role its EKS access entry) — then
   EXP-54.
4. EXP-50..55, incl. flipping the reaper's DRY_RUN (EXP-55) and the
   presigned-URL-under-IRSA round-trip (EXP-50).
5. Follow-ups registered in AWS_SESSION.md "Known gaps": lab-obs
   postgres-exporter ExternalSecret, `patches/netpols-aws.yaml` once
   VPC-CNI enforcement is exercised, WAF for auth rate-limiting,
   auth-service RDS TLS check, uniform Secrets-Manager migration for
   rabbitmq/mongo/jwt.

The original sequencing below is kept as the record of what each step
meant — read it with the state above in mind.

Order matters: 0 → 1 → 2 → 3/4 (parallel-ok) → 5 → 6 → 7 → 8 → 9.

## 0 — Account (manual, owner does this)
Dedicated account, SSO/IAM profile `lab`, ONE region, `terraform.tfvars`
from the example. Without this, stop.

## 1 — Validate & bootstrap
`brew install terraform`; `terraform fmt -check -recursive deploy/aws` and
`terraform validate` in all three stacks (fix syntax drift from authoring);
apply backend-bootstrap; wire base/session `terraform init
-backend-config` invocations into the make targets (they currently assume
an initialized dir — add `aws-init` target doing both inits with the
bucket name from bootstrap output).

## 2 — Base stack
Apply. Delegate NS records. Seed ECR (`make images
REGISTRY=<ecr>/coppice-lab TAG=$(git rev-parse --short HEAD)` after ECR
docker login — note `make images` takes REGISTRY already; repo names must
match `coppice-lab/<img>`: adjust the images target or the ECR repo names
to line up, simplest is a `REPO_PREFIX` make var). Budget email lands.

## 3 — Budget → ntfy (optional this phase)
SNS topic + Lambda POSTing to ntfy (payload mapping mirrors
`scripts/obs/ntfy-relay/main.go`). Email alone is acceptable to exit v5.

## 4 — Reaper
Implement `deploy/aws/base/reaper/reaper.py` per its docstring; add
`make aws-reaper-pack` (zip → `reaper.zip`, gitignored); tighten IAM to the
exact delete surface; keep `DRY_RUN=true` until EXP-55: dry-run list →
decoy resource (e.g. tagged elastic IP) reaped → flip flag.

## 5 — Session stack fill-ins (main work)
Each TODO block in `deploy/aws/session/main.tf` names its module + pinned
version + inputs:
- **5.2 EKS** terraform-aws-modules/eks ~>21, IRSA on, managed node group
  (2 min / var.node_count desired), then helm_release (separate
  `addons.tf`, kubernetes+helm providers from EKS outputs) for
  aws-load-balancer-controller, external-secrets, external-dns — pin
  charts at implementation time (`helm search repo`), record pins in
  AWS_SESSION.md.
- **5.3 RDS** rds module ~>6, postgres 15, t4g.micro, `multi_az` variable
  default false (true only for the EXP-53 failover session); master creds
  → Secrets Manager; api_db/auth_db created by pointing the existing
  migration Jobs at RDS (overlay patch) — NOT hand psql.
- **5.4 IRSA for api-service** — S3 CRUD on the documents bucket; presigned
  GETs work with IRSA creds (verify explicitly: EXP-50 asserts a presigned
  URL round-trip; note S3 vs MinIO presign behavior diffs in the write-up).
- **5.5 ACM + external-dns** — wildcard cert DNS-validated into the base
  zone; external-dns manages api./grafana. records from Ingress hosts.

## 6 — AWS overlay
Execute the 6 numbered patches listed in
`deploy/k8s/overlays/aws/kustomization.yaml` (images transformer, minio
removal + S3 env/IRSA SA, ExternalSecret CRs replacing init-secrets
Secrets **keeping the same Secret names/keys**, ALB ingress + ACM, gp3
storage, VPC-CNI network-policy addon flag). Keep drift-check green: the
aws overlay is NOT drift-checked (kind-local is), but CI should at least
`kustomize build` it — add to the drift-check job's render list.

## 7 — Wire `make aws-up` fully
Sequence in the target (replacing the TODO echo): terraform apply →
`aws eks update-kubeconfig` → `kubectl apply -k deploy/k8s/overlays/aws`
(with the cert-manager/ingress-nginx vendor steps SKIPPED — ALB replaces
them) → obs install (`obs-up.sh` works unchanged; OpenSearch optional per
session — make it `OBS_LOGS=0` env-gated) → checkpoint loop printing the
AWS_SESSION.md milestones. `aws-down`: destroy + `scripts/aws/assert-clean.sh`.

## 8 — assert-clean script
`scripts/aws/assert-clean.sh`: resourcegroupstaggingapi query
`project=coppice-lab` minus the persistent allowlist (tfstate bucket, lock
table, ECR repos, zone, budget, reaper lambda+role+rule) → non-empty ⇒
exit 1 listing ARNs. This is EXP-50's teardown assertion.

## 9 — CI phase 3 (ADR-006.7, ADR-010.2)
`.github/workflows/deploy-aws.yml`: OIDC role (create in base stack:
`aws_iam_openid_connect_provider` for token.actions.githubusercontent.com
+ role trust-scoped to this repo+branch) → build+push images to ECR →
`kubectl apply -k` deploy job. Triggers: manual (workflow_dispatch) +
on-tag. Keep laptop `make images` as fallback. Rollback = re-deploy
previous tag (EXP-54).

## Exit = EXP-50..55 (phase doc) — each with a cost line
Cheapest sane order: 50 (lifecycle+cost baseline) → 51 (catalog) → 55
(reaper) → 52 (node kill) → 53 (RDS failover, multi_az session) → 54
(pipeline). Update AWS_SESSION.md's ~ numbers with actuals; then tag
`lab-v5.0`.
