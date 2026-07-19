# Phase v5 handoff — AWS track

**Audience:** the session that finishes v5. Decisions are settled
(ADR-006); this file sequences the work. The skeleton you inherit:
`deploy/aws/{backend-bootstrap,base,session}` (bootstrap + base are
essentially complete; session has settled TODO blocks),
`deploy/k8s/overlays/aws` (renders, patches pending), `make
aws-plan/aws-up/aws-down` stubs, `documentation/deployment/AWS_SESSION.md`
draft. **Nothing has ever been applied — terraform was not installed at
authoring time; expect first-`terraform validate` fixes as your step 1.**

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
