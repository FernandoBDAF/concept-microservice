# Outputs the aws overlay + `make aws-up`/AWS_SESSION.md consume via
# `terraform output -json` (HANDOFF §5). Everything the cluster wiring needs to
# reach the managed services lives here.

output "region" {
  value = var.aws_region
}

# ── EKS ──────────────────────────────────────────────────────────────────────
output "cluster_name" {
  value = module.eks.cluster_name
}

output "cluster_endpoint" {
  value = module.eks.cluster_endpoint
}

output "oidc_provider_arn" {
  value = module.eks.oidc_provider_arn
}

# ── ECR (registry host; base stack owns the repos) ───────────────────────────
# Constructed from account+region so this stack needs no cross-stack read.
# `make images REGISTRY=<this>/coppice-lab` and the overlay images transformer
# both consume it.
output "ecr_registry" {
  value = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com"
}

# ── S3 + api-service IRSA (HANDOFF §5.4) ─────────────────────────────────────
output "documents_bucket" {
  value = aws_s3_bucket.documents.bucket
}

output "api_service_irsa_role_arn" {
  value       = aws_iam_role.api_service.arn
  description = "annotate the lab-core api-service ServiceAccount with this (eks.amazonaws.com/role-arn)"
}

output "graphrag_service_irsa_role_arn" {
  value       = aws_iam_role.graphrag_service.arn
  description = "annotate the lab-core graphrag-service ServiceAccount with this (eks.amazonaws.com/role-arn)"
}

# ── RDS (HANDOFF §5.3) ───────────────────────────────────────────────────────
output "rds_endpoint" {
  value = module.rds.db_instance_endpoint
}

output "rds_address" {
  value       = module.rds.db_instance_address
  description = "host only (no :port) — the overlay patches this into the api/auth DSNs"
}

output "rds_port" {
  value = module.rds.db_instance_port
}

output "postgres_secret_arn" {
  value       = aws_secretsmanager_secret.postgres_credentials.arn
  description = "Secrets Manager JSON {POSTGRES_PASSWORD, AUTH_DB_PASSWORD}; the overlay's ExternalSecret source"
}

output "postgres_secret_name" {
  value       = aws_secretsmanager_secret.postgres_credentials.name
  description = "stable name the overlay's ExternalSecret references in its remoteRef"
}

# ── ACM (HANDOFF §5.5) ───────────────────────────────────────────────────────
output "acm_certificate_arn" {
  value       = aws_acm_certificate_validation.wildcard.certificate_arn
  description = "wildcard *.<lab_domain> cert; the ALB controller auto-discovers it by ingress host — the ingress does NOT reference this ARN"
}

output "zone_id" {
  value = data.aws_route53_zone.lab.zone_id
}

output "lab_domain" {
  value       = var.lab_domain
  description = "for make aws-deploy's LAB_DOMAIN_PLACEHOLDER substitution (ingress hosts)"
}
