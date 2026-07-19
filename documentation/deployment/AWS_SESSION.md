# AWS session runbook (ADR-006 · phase v5)

> **Status: draft** — authored with the v5 skeleton; numbers marked ~ are
> estimates to replace with measured values on the first real session
> (EXP-50 records actuals).

A *session* is the unit of AWS usage: `make aws-up` → drills → `make
aws-down`, same day. Between sessions only the base stack exists (tfstate,
ECR images, Route53 zone, budget, reaper) at ≈ $0/month.

## One-time setup (step 0 — manual)

1. Dedicated AWS account; enable SSO or create an IAM user; profile `lab`
   in `~/.aws/config`, pinned region (ADR-006.5).
2. `brew install terraform` (≥1.9). Copy
   `deploy/aws/terraform.tfvars.example` → `deploy/aws/terraform.tfvars`.
3. `cd deploy/aws/backend-bootstrap && terraform init && terraform apply
   -var-file=../terraform.tfvars` — note the `state_bucket` output.
4. `cd ../base && terraform init -backend-config=...` (README) `&&
   terraform apply -var-file=../terraform.tfvars`; delegate your domain's
   NS records to the `zone_ns` output.
5. `make images REGISTRY=<ecr_registry>/coppice-lab TAG=<sha>` after
   `aws ecr get-login-password | docker login ...` — first push seeds ECR.

## Session start (~20 min)

```
make aws-plan    # review; no surprises policy
make aws-up      # terraform apply session/ + kubeconfig + kubectl apply -k overlays/aws + obs
```

Checkpoints (the make target prints them): VPC+EKS Ready (~12 min) → RDS
available (~5 min) → migrations Jobs Complete → `kubectl -n lab-core get
pods` all Ready → `curl https://api.<domain>/health` 200 via ALB+ACM.

## Verify

Scored smoke against the session: `make experiment E=exp-02` (API_URL
override per experiment needs — HANDOFF §7), then the drill of the day
(EXP-50..55 catalog).

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

tfstate (S3+lock), ECR images, Route53 zone, budget+alarms, reaper. Each
is pennies or free. Everything else has `stack=session` + `ttl` tags.
