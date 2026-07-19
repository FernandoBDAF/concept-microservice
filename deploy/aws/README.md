# AWS track (ADR-006) — Terraform skeleton

Three stacks, three lifecycles (ADR-006.4: idle month ≈ $0):

| Stack | Dir | Lifecycle | Contents |
|---|---|---|---|
| backend-bootstrap | `backend-bootstrap/` | once, ever | tfstate S3 bucket + DynamoDB lock table |
| base | `base/` | persistent, near-$0 | ECR repos, Route53 zone, budget+alarms, TTL reaper |
| session | `session/` | `make aws-up` ↔ `make aws-down` | VPC, EKS, RDS, S3, ACM/ALB — everything billable-by-the-hour |

Prerequisites (manual, step 0 — see `documentation/phases/v5-HANDOFF.md`):
dedicated AWS account, one pinned region, an IAM role/SSO profile for
tooling, `terraform` ≥ 1.9 installed, and `deploy/aws/terraform.tfvars`
created from `terraform.tfvars.example` (never committed).

Nothing here has ever been applied — this is the authored skeleton
(expedited v5); `terraform validate` each stack after installing terraform,
then follow the HANDOFF order: bootstrap → base → session.
