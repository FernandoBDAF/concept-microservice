# Session stack (ADR-006): everything billable-by-the-hour. `make aws-up`
# applies it, `make aws-down` destroys it — NOTHING here may survive a
# session (the reaper backstops leaks via the ttl tag below).
#
# SKELETON: module choices + wiring are settled; the TODO(v5) blocks are
# fill-ins, not design questions. HANDOFF §5 walks each one with the exact
# module inputs to start from. `terraform validate` after each fill-in.

terraform {
  required_version = ">= 1.9"
  required_providers {
    aws  = { source = "hashicorp/aws", version = "~> 6.0" }
    time = { source = "hashicorp/time", version = "~> 0.12" }
  }
  backend "s3" {
    key     = "session/terraform.tfstate"
    encrypt = true
  }
}

variable "aws_region" { type = string }
variable "aws_profile" { type = string }
variable "lab_domain" { type = string }
variable "budget_limit" { type = number }
variable "alert_email" { type = string }

variable "session_ttl_hours" {
  type        = number
  default     = 8
  description = "ttl tag = now + this; the reaper kills leaks after it"
}

variable "node_instance_type" {
  type    = string
  default = "t3.medium" # 2-3 smallish nodes (phase doc); Spot deferred
}

variable "node_count" {
  type    = number
  default = 3
}

provider "aws" {
  region  = var.aws_region
  profile = var.aws_profile
  default_tags {
    tags = {
      project = "coppice-lab"
      stack   = "session"
      ttl     = tostring(time_static.session_start.unix + var.session_ttl_hours * 3600)
    }
  }
}

resource "time_static" "session_start" {}

# ── VPC — community module (ADR-006.1: modules over hand-rolled) ─────────────
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 6.0"

  name = "coppice-lab"
  cidr = "10.42.0.0/16"
  azs  = ["${var.aws_region}a", "${var.aws_region}b"]

  private_subnets = ["10.42.1.0/24", "10.42.2.0/24"]
  public_subnets  = ["10.42.101.0/24", "10.42.102.0/24"]

  # single NAT: sessions are short; HA-NAT is real-world money for no drill
  enable_nat_gateway = true
  single_nat_gateway = true

  public_subnet_tags  = { "kubernetes.io/role/elb" = "1" }
  private_subnet_tags = { "kubernetes.io/role/internal-elb" = "1" }
}

# ── EKS — managed node group (ADR-006.2) ─────────────────────────────────────
# TODO(v5, HANDOFF §5.2): fill in with terraform-aws-modules/eks/aws ~> 21.0:
#   cluster_name coppice-lab, cluster_version pinned (check latest-1),
#   vpc_id/subnets from module.vpc.private_subnets,
#   eks_managed_node_groups = { lab = { instance_types=[var.node_instance_type],
#     min_size=2, max_size=var.node_count, desired_size=var.node_count } },
#   enable_irsa = true (api-service S3 role + external-secrets + ALB
#   controller all need OIDC), cluster_endpoint_public_access = true.
# Addons: aws-load-balancer-controller + external-secrets via helm_release
# (separate providers file) — HANDOFF lists the pinned chart versions.

# ── RDS Postgres — both DBs on one instance (ADR-006.3) ──────────────────────
# TODO(v5, HANDOFF §5.3): terraform-aws-modules/rds/aws ~> 6.0:
#   engine postgres 15, db.t4g.micro, 20GB gp3, single-AZ,
#   backup_retention_period = 1 (EXP-53 needs failover-capable settings:
#   actually use Multi-AZ = false but reboot-with-failover requires
#   Multi-AZ — decide per EXP-53: enable multi_az ONLY for that drill's
#   session via a variable `multi_az`, default false for cost),
#   parameter group: max_connections sized per pool settings,
#   creates api_db + auth_db via a post-provision null_resource psql or the
#   k8s migration jobs pointed at it (preferred — same migration path).
#   Master password → Secrets Manager (aws_secretsmanager_secret) consumed
#   by external-secrets (ADR-009.3).

# ── S3 replacing MinIO (ADR-006.3) ──────────────────────────────────────────
resource "aws_s3_bucket" "documents" {
  bucket_prefix = "coppice-lab-documents-"
  force_destroy = true # session bucket: aws-down must not strand objects
}

resource "aws_s3_bucket_public_access_block" "documents" {
  bucket                  = aws_s3_bucket.documents.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# TODO(v5, HANDOFF §5.4): IRSA role for api-service (s3:GetObject/PutObject/
# DeleteObject on this bucket + presigned URL support needs no extra perms),
# trust policy bound to the api-service ServiceAccount via the EKS OIDC
# provider output.

# ── ACM cert for *.lab_domain + DNS records (ADR-006.6) ──────────────────────
# TODO(v5, HANDOFF §5.5): aws_acm_certificate (DNS validation into the base
# stack's zone via data.aws_route53_zone lookup on var.lab_domain),
# wildcard *.${var.lab_domain}; ALB ingress records are created by
# external-dns OR explicit aws_route53_record alias entries for
# api./grafana. once the ALB exists — external-dns chosen (HANDOFF pins it).

output "documents_bucket" { value = aws_s3_bucket.documents.bucket }
# TODO(v5): outputs for cluster_name, cluster_endpoint, rds_endpoint,
# secrets ARNs — the aws overlay + AWS_SESSION.md consume these via
# `terraform output -json` (make aws-kubeconfig target).
