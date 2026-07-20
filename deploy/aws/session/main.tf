# Session stack (ADR-006): everything billable-by-the-hour. `make aws-up`
# applies it, `make aws-down` destroys it — NOTHING here may survive a
# session (the reaper backstops leaks via the ttl tag below).
#
# SKELETON: module choices + wiring are settled; the TODO(v5) blocks are
# fill-ins, not design questions. HANDOFF §5 walks each one with the exact
# module inputs to start from. `terraform validate` after each fill-in.
#
# Layout (v5 fill-in): providers.tf (kubernetes/helm from EKS outputs),
# addons.tf (helm_release + addon IRSA), outputs.tf (overlay consumes them).

terraform {
  required_version = ">= 1.9"
  required_providers {
    aws  = { source = "hashicorp/aws", version = "~> 6.0" }
    time = { source = "hashicorp/time", version = "~> 0.12" }
    # addons.tf providers (kubernetes/helm from EKS outputs) + random for the
    # RDS master password. Terraform allows only ONE required_providers block
    # per module, so the addon providers live here, not in providers.tf.
    kubernetes = { source = "hashicorp/kubernetes", version = "~> 2.30" }
    helm       = { source = "hashicorp/helm", version = "~> 2.15" }
    random     = { source = "hashicorp/random", version = "~> 3.6" }
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

variable "cluster_version" {
  type = string
  # latest-1 (ADR-006.2): pin one minor behind the newest EKS release so the
  # control plane is battle-tested. Bump in tfvars each session as EKS moves.
  default     = "1.33"
  description = "EKS control-plane version; keep at latest-1"
}

variable "deploy_role_arn" {
  type = string
  # base stack output `oidc_deploy_role_arn` (WP4: IAM role coppice-lab-deploy,
  # assumed by the GitHub Actions OIDC pipeline). Empty ⇒ skip the access entry
  # so `terraform validate`/first apply works before the base stack is applied.
  default     = ""
  description = "base stack output oidc_deploy_role_arn; grants the CI deploy role cluster access"
}

variable "multi_az" {
  type = bool
  # Cost default is single-AZ. EXP-53 (RDS failover drill) is the ONLY session
  # that flips this true — reboot-with-failover requires a standby (HANDOFF §5.3).
  default     = false
  description = "RDS Multi-AZ; true only for the EXP-53 failover session"
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
# HANDOFF §5.2: cluster coppice-lab, cluster_version latest-1, subnets from
# module.vpc, one managed node group (2 min / var.node_count desired), IRSA +
# public endpoint. Addons (ALB controller / external-secrets / external-dns)
# live in addons.tf with providers built from these outputs.
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 21.0"

  name               = "coppice-lab"
  kubernetes_version = var.cluster_version

  # Public endpoint so `make aws-up` (laptop) can kubectl straight after apply;
  # private access too so in-VPC traffic (nodes) never leaves the VPC.
  endpoint_public_access  = true
  endpoint_private_access = true

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  # The applying principal (make/CI role) gets cluster-admin via an access
  # entry so kubectl works immediately post-apply (v21 API auth mode).
  enable_cluster_creator_admin_permissions = true

  # CI deploy role (WP4 base-stack oidc.tf → oidc_deploy_role_arn) needs
  # in-cluster authz so `kubectl apply -k` works with no laptop creds. Gated
  # on the var so validate/apply pass before the base stack exists (empty ⇒ {}).
  # TODO(v5): scope narrower than cluster-admin once EXP-54 pins the exact verbs.
  access_entries = var.deploy_role_arn == "" ? {} : {
    ci_deploy = {
      principal_arn = var.deploy_role_arn
      policy_associations = {
        admin = {
          policy_arn   = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
          access_scope = { type = "cluster" }
        }
      }
    }
  }

  eks_managed_node_groups = {
    lab = {
      instance_types = [var.node_instance_type]
      min_size       = 2
      max_size       = var.node_count
      desired_size   = var.node_count
    }
  }

  # VPC-CNI network-policy agent: the base netpols (deploy/k8s/base/netpols)
  # need it enforced on EKS (kind's CNI does this natively). HANDOFF §6.6.
  addons = {
    vpc-cni = {
      configuration_values = jsonencode({
        enableNetworkPolicy = "true"
      })
    }
    coredns    = {}
    kube-proxy = {}
  }
}

# ── RDS Postgres — both DBs on one instance (ADR-006.3) ──────────────────────
# HANDOFF §5.3: postgres 15, db.t4g.micro, 20GB gp3, backup_retention 1,
# multi_az via var (default false). Master creds → Secrets Manager under the
# EXACT keys the cluster's `postgres-credentials` Secret uses (POSTGRES_PASSWORD,
# AUTH_DB_PASSWORD) so WP2's ExternalSecret is a straight passthrough. api_db /
# auth_db + the auth_user role are created by the migration Jobs pointed here
# (overlay patch, NOT this file — see wp1 integration notes for the gap).
resource "random_password" "postgres" {
  length  = 32
  special = false # keep DSN URL-safe (no @/: to escape in the connection string)
}

resource "random_password" "auth_db" {
  length  = 32
  special = false
}

# Master creds mirror the k8s Secret `postgres-credentials` key-for-key
# (recon: scripts/cluster/init-secrets.sh + api/auth deployments). external-
# secrets (WP2) does `extract` on this JSON → identical Secret, deployments
# unchanged. NOTE: cluster builds the DSN inline from host+password, so the
# host is patched by the overlay (RDS endpoint), NOT stored here.
resource "aws_secretsmanager_secret" "postgres_credentials" {
  name = "coppice-lab/session/postgres-credentials"
  # recovery window 0: aws-down deletes immediately so the next session can
  # re-create the same name without hitting the 7-day soft-delete window.
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "postgres_credentials" {
  secret_id = aws_secretsmanager_secret.postgres_credentials.id
  secret_string = jsonencode({
    POSTGRES_PASSWORD = random_password.postgres.result
    AUTH_DB_PASSWORD  = random_password.auth_db.result
  })
}

# Postgres reachable only from the EKS nodes on 5432 (least privilege).
resource "aws_security_group" "rds" {
  name_prefix = "coppice-lab-rds-"
  description = "coppice-lab postgres — 5432 from EKS nodes only"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "postgres from EKS node group"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [module.eks.node_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  lifecycle { create_before_destroy = true }
}

module "rds" {
  source  = "terraform-aws-modules/rds/aws"
  version = "~> 6.0"

  identifier = "coppice-lab"

  engine               = "postgres"
  engine_version       = "15"
  family               = "postgres15"
  major_engine_version = "15"
  instance_class       = "db.t4g.micro"

  allocated_storage = 20
  storage_type      = "gp3"

  # api_db is the initial DB; auth_db is created by the auth migration path
  # (overlay). Master user `postgres` matches the api-service DSN user.
  db_name  = "api_db"
  username = "postgres"

  # We own the password (random_password above) so the Secrets Manager keys
  # match the cluster's — NOT the module's manage_master_user_password (which
  # would emit an AWS-managed secret with username/password keys we can't rename).
  manage_master_user_password = false
  password                    = random_password.postgres.result
  port                        = 5432

  multi_az               = var.multi_az
  vpc_security_group_ids = [aws_security_group.rds.id]

  create_db_subnet_group = true
  subnet_ids             = module.vpc.private_subnets

  backup_retention_period = 1
  skip_final_snapshot     = true # session DB: aws-down must not strand a snapshot
  deletion_protection     = false

  create_db_parameter_group = true
  parameter_group_name      = "coppice-lab-postgres15"
  # max_connections sized for the api + auth pools (small lab). t4g.micro's
  # memory-derived default is ~112; pin 100 explicitly. TODO(v5): tune against
  # EXP-53 pool-recovery findings if the failover drill starves connections.
  parameters = [
    {
      name  = "max_connections"
      value = "100"
    }
  ]
}

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

# ── IRSA for api-service — S3 CRUD on the documents bucket (HANDOFF §5.4) ─────
# Trust bound to the api-service ServiceAccount in lab-core (recon: base
# manifests run api-service in namespace lab-core; WP2 must name the SA
# `api-service` and annotate it with this role ARN). Presigned GETs need no
# extra perms — the SDK signs locally with the assumed-role creds.
data "aws_iam_policy_document" "api_service_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [module.eks.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${module.eks.oidc_provider}:sub"
      values   = ["system:serviceaccount:lab-core:api-service"]
    }

    condition {
      test     = "StringEquals"
      variable = "${module.eks.oidc_provider}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "api_service" {
  name               = "coppice-lab-api-service"
  assume_role_policy = data.aws_iam_policy_document.api_service_assume.json
}

data "aws_iam_policy_document" "api_service_s3" {
  statement {
    sid       = "BucketList"
    effect    = "Allow"
    actions   = ["s3:ListBucket", "s3:GetBucketLocation"]
    resources = [aws_s3_bucket.documents.arn]
  }
  statement {
    sid       = "ObjectCRUD"
    effect    = "Allow"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
    resources = ["${aws_s3_bucket.documents.arn}/*"]
  }
}

resource "aws_iam_role_policy" "api_service_s3" {
  name   = "documents-bucket-crud"
  role   = aws_iam_role.api_service.id
  policy = data.aws_iam_policy_document.api_service_s3.json
}

# graphrag-service also reads/writes the documents bucket (base netpols allow
# it to minio today) — own role, same S3 policy, trust bound to its own SA
# (the aws overlay creates lab-core/graphrag-service and annotates it).
data "aws_iam_policy_document" "graphrag_service_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [module.eks.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${module.eks.oidc_provider}:sub"
      values   = ["system:serviceaccount:lab-core:graphrag-service"]
    }

    condition {
      test     = "StringEquals"
      variable = "${module.eks.oidc_provider}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "graphrag_service" {
  name               = "coppice-lab-graphrag-service"
  assume_role_policy = data.aws_iam_policy_document.graphrag_service_assume.json
}

resource "aws_iam_role_policy" "graphrag_service_s3" {
  name   = "documents-bucket-crud"
  role   = aws_iam_role.graphrag_service.id
  policy = data.aws_iam_policy_document.api_service_s3.json
}

# ── ACM cert for *.lab_domain + DNS validation (ADR-006.6, HANDOFF §5.5) ──────
# Wildcard cert DNS-validated into the base stack's zone. external-dns manages
# the ingress records (api./grafana.) once the ALB exists — its IRSA role
# (addons.tf) is scoped to this zone.
data "aws_route53_zone" "lab" {
  name         = var.lab_domain
  private_zone = false
}

resource "aws_acm_certificate" "wildcard" {
  domain_name               = "*.${var.lab_domain}"
  subject_alternative_names = [var.lab_domain] # apex too, so grafana/api hosts + bare domain both covered
  validation_method         = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_route53_record" "acm_validation" {
  for_each = {
    for dvo in aws_acm_certificate.wildcard.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }

  zone_id         = data.aws_route53_zone.lab.zone_id
  name            = each.value.name
  type            = each.value.type
  records         = [each.value.record]
  ttl             = 60
  allow_overwrite = true
}

resource "aws_acm_certificate_validation" "wildcard" {
  certificate_arn         = aws_acm_certificate.wildcard.arn
  validation_record_fqdns = [for r in aws_route53_record.acm_validation : r.fqdn]
}
