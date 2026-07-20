# AWS track (ADR-006) — Terraform stacks

Three stacks, three lifecycles (ADR-006.4: idle month ≈ $0):

| Stack | Dir | Lifecycle | Contents |
|---|---|---|---|
| backend-bootstrap | `backend-bootstrap/` | once, ever | tfstate S3 bucket + DynamoDB lock table |
| base | `base/` | persistent, near-$0 | ECR repos, Route53 zone, budget+alarms (opt. →ntfy), TTL reaper, CI OIDC role |
| session | `session/` | `make aws-up` ↔ `make aws-down` | VPC, EKS, RDS, S3, ACM/ALB — everything billable-by-the-hour |

Prerequisites (manual, step 0 — see `documentation/phases/v5-HANDOFF.md`):
dedicated AWS account, one pinned region, an IAM role/SSO profile for
tooling, `terraform` ≥ 1.9 installed, and `deploy/aws/terraform.tfvars`
created from `terraform.tfvars.example` (never committed).

State (2026-07-19, v5 execution pass): all three stacks are filled in and
`terraform fmt`/`validate` clean (aws ~>6; eks 21.x / rds 6.x / iam 5.x /
vpc 6.x modules resolved live from the registry). **Nothing has been
applied** — that needs the step-0 account. Flow once it exists:
`terraform apply` in backend-bootstrap → `make aws-init` (points base +
session at the S3 backend) → `make aws-base-pack` (lambda zips) → apply
base → `make aws-up` / `make aws-down` per session. Runbook:
`documentation/deployment/AWS_SESSION.md`.
