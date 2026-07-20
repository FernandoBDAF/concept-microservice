# AWS session runbook (ADR-006 · phase v5)

> **Status: code-complete, never applied** — updated in the v5 execution
> pass (2026-07-19): all stacks/overlay/pipeline are implemented and
> statically verified, but no AWS account has run them. Numbers marked ~
> are estimates to replace with measured values on the first real session
> (EXP-50 records actuals). First-session verification points are listed
> under "Known gaps" below.

A *session* is the unit of AWS usage: `make aws-up` → drills → `make
aws-down`, same day. Between sessions only the base stack exists (tfstate,
ECR images, Route53 zone, budget, reaper) at ≈ $0/month.

## One-time setup (step 0 — manual)

1. Dedicated AWS account; enable SSO or create an IAM user; profile `lab`
   in `~/.aws/config`, pinned region (ADR-006.5).
2. Install `terraform` (≥1.9) + `aws` CLI. Copy
   `deploy/aws/terraform.tfvars.example` → `deploy/aws/terraform.tfvars`.
3. `cd deploy/aws/backend-bootstrap && terraform init && terraform apply
   -var-file=../terraform.tfvars`, then `make aws-init` — it reads the
   bootstrap outputs and points base+session at the S3 backend.
4. `make aws-base-pack` (lambda zips), then `cd deploy/aws/base &&
   terraform apply -var-file=../terraform.tfvars`; delegate your domain's
   NS records to the `zone_ns` output.
5. `make images REGISTRY=<ecr_registry>/coppice-lab TAG=<sha>` after
   `aws ecr get-login-password | docker login ...` — first push seeds ECR.
6. Pipeline (EXP-54): set GitHub repo *variables* `AWS_DEPLOY_ROLE_ARN`
   (base output `oidc_deploy_role_arn`) and `AWS_REGION`; per session, set
   `deploy_role_arn` in tfvars to that same ARN so the cluster grants the
   CI role an access entry. `.github/workflows/deploy-aws.yml` then
   builds+pushes on tag `lab-v*` or dispatch; deploy leg is dispatch-gated.

## Session start (~20 min)

```
make aws-plan    # review; no surprises policy
make aws-up      # terraform apply session/ → aws-deploy (kubeconfig, overlay
                 # with images+placeholders substituted, secrets, obs, checkpoints)
```

Session knobs: `TAG=<sha>` picks the image tag (default: current git short
SHA — must exist in ECR); `OBS_LOGS=1` opts into OpenSearch/fluent-bit
(default off on AWS — RAM/cost); tfvars `multi_az=true` only for the
EXP-53 failover session. `make aws-deploy` mutates
`deploy/k8s/overlays/aws/kustomization.yaml` via `kustomize edit set
image` — don't commit that change.

Checkpoints (`scripts/aws/session-checkpoints.sh` prints them): VPC+EKS
Ready (~12 min) → RDS available (~5 min) → rds-bootstrap + migration Jobs
Complete → `kubectl -n lab-core get pods` all Ready → ALB provisioned +
external-dns records → `curl https://api.<domain>/health` 200 via ALB+ACM.

Addon chart pins (helm, recorded at implementation 2026-07-19 — bump
deliberately in `deploy/aws/session/addons.tf`):
`aws-load-balancer-controller 3.4.2` · `external-secrets 2.8.0` (CRs use
`external-secrets.io/v1`) · `external-dns 1.21.1`.

## Verify

Smoke against the session: `curl https://api.<domain>/health` + EXP-02's
manual steps against the ALB hosts, then the drill of the day (EXP-50..55
catalog). NOTE: the scored runner (`make experiment E=exp-02`, v4)
currently targets compose-local URLs — pointing it at an EKS session needs
a base-URL override in the experiment defs (see Known gaps).

## Cost check (every session)

Cost Explorer → filter tag `project=coppice-lab`, group by `stack`.
Record in the session write-up: date, duration, $ actual. ~Expected:
EKS control plane $0.10/h + 3×t3.medium ~$0.12/h + NAT ~$0.045/h + RDS
t4g.micro ~$0.016/h ≈ **$0.65/h ≈ $5 for a long evening**.

## Session end

```
make aws-down    # terraform destroy session/ with confirmation
```

Then verify $0 residuals: `bash scripts/aws/assert-clean.sh` (HANDOFF §8 —
lists any resource still tagged project=coppice-lab that isn't in the
persistent allowlist; the reaper is the backstop, not the primary path).

## What survives a session (and why that's all)

tfstate (S3+lock), ECR images, Route53 zone, budget+alarms (+optional
budget→ntfy SNS/Lambda), reaper, the CI OIDC provider+role. Each is
pennies or free. Everything else has `stack=session` + `ttl` tags.

## Known gaps — verify on the first real session

Registered here and in `documentation/phases/v5-HANDOFF.md`; none are
blockers for `make aws-up` itself:

- **Presigned URLs under IRSA** (EXP-50): api-service/graphrag now fall
  back to ambient (IRSA) credentials when static keys are absent —
  presigning with temporary creds embeds the session token; verify the
  round-trip against real S3.
- **postgres-exporter is down on AWS**: `lab-obs/postgres-credentials` is
  deliberately not seeded (ExternalSecret owns postgres creds); the
  exporter needs its own ExternalSecret + RDS host wiring (follow-up).
- **Netpols are enforced on EKS** (VPC-CNI network-policy agent is on) but
  the base allows are kind-shaped: ALB ingress (ipBlock) + RDS/S3 egress
  need an aws netpol patch (`patches/netpols-aws.yaml` follow-up) — if
  traffic is unexpectedly blocked in the first session, this is why.
- **Auth rate limiting** (ADR-009.5) has no ALB equivalent — needs WAF
  later; dropped on AWS for now.
- **auth-service ↔ RDS TLS**: if `rds.force_ssl` is on, the pg client may
  need a TLS env — verify at first deploy.
- **rabbitmq/mongo/jwt secrets** stay init-secrets-seeded on AWS
  (`SKIP_POSTGRES=1` guards the postgres one); uniform Secrets-Manager +
  ExternalSecret migration is a registered follow-up.
- **Scored runner is compose-local** (v4): experiment YAMLs hardcode
  `http://localhost:8080`-style URLs and `scripts/experiments/run.py` has
  no base-URL override — EXP-51 ("catalog on EKS") needs one added (env,
  e.g. `EXPERIMENT_BASE_URL`, rewriting hosts per def) before the scored
  subset can run against `api.<domain>`.
- **rabbitmq guest password ⇄ definitions.json** (v4 deferral, ADR-008.4):
  applies unchanged on EKS — the broker boots from the committed
  definitions with guest/guest until definitions are regenerated with the
  rotated password; same caveat, same fix path as kind.
