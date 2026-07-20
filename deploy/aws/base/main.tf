# Persistent base stack (ADR-006.4): everything here must idle at ≈ $0.
# ECR repos, Route53 zone, budget alarms, TTL reaper. Applied rarely;
# destroyed never (except account teardown).
#
# terraform init -backend-config="bucket=<state_bucket from bootstrap>" \
#                -backend-config="dynamodb_table=coppice-lab-tf-lock" \
#                -backend-config="region=<aws_region>"
# terraform apply -var-file=../terraform.tfvars

terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 6.0" }
  }
  backend "s3" {
    key     = "base/terraform.tfstate"
    encrypt = true
    # bucket/dynamodb_table/region via -backend-config (account-specific)
  }
}

variable "aws_region" { type = string }
variable "aws_profile" { type = string }
variable "lab_domain" { type = string }
variable "budget_limit" { type = number }
variable "alert_email" { type = string }

# HANDOFF §3: budget → ntfy is optional this phase. Empty (default) keeps the
# email-only v0 exit path — the whole SNS + notifier-Lambda path counts to 0.
# Set to a full ntfy topic URL (e.g. https://ntfy.sh/coppice-lab-budget) to
# also fan budget breaches out to ntfy.
variable "ntfy_topic_url" {
  type    = string
  default = ""
}

provider "aws" {
  region  = var.aws_region
  profile = var.aws_profile
  default_tags {
    tags = { project = "coppice-lab", stack = "base" }
  }
}

# ── ECR: one repo per image `make images` pushes (incl. v3/v4 additions) ─────
locals {
  images = [
    "api-service", "auth-service", "graphrag-service",
    "email-worker", "image-worker", "profile-worker",
    "loadgen", # v4 flood generator (ADR-004.4) — EXP-51 drills need it on EKS
    "ntfy-relay", "hello-guest-web", "hello-guest-worker",
  ]
}

resource "aws_ecr_repository" "svc" {
  for_each             = toset(local.images)
  name                 = "coppice-lab/${each.key}"
  image_tag_mutability = "MUTABLE" # dev tags; sessions pin by digest/tag
  force_delete         = true
}

resource "aws_ecr_lifecycle_policy" "svc" {
  for_each   = aws_ecr_repository.svc
  repository = each.value.name
  # keep last 10 images — storage stays in cents territory
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "keep last 10"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 10
      }
      action = { type = "expire" }
    }]
  })
}

# ── Route53 zone (ADR-006.6) — delegate lab_domain's NS here ─────────────────
resource "aws_route53_zone" "lab" {
  name    = var.lab_domain
  comment = "coppice lab (ADR-006.6); survives sessions"
}

# ── Budget + alarms (ADR-006.4): 50/80/100% notifications ───────────────────
resource "aws_budgets_budget" "monthly" {
  name         = "coppice-lab-monthly"
  budget_type  = "COST"
  limit_amount = tostring(var.budget_limit)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  dynamic "notification" {
    for_each = [50, 80, 100]
    content {
      comparison_operator        = "GREATER_THAN"
      threshold                  = notification.value
      threshold_type             = "PERCENTAGE"
      notification_type          = "ACTUAL"
      subscriber_email_addresses = [var.alert_email]
      # HANDOFF §3: ntfy fan-out via SNS. Email is the v0 channel and always
      # present; the SNS topic (→ notifier Lambda → ntfy) is added only when
      # ntfy_topic_url is set. Splat over the count-guarded topic gives [] when
      # disabled, so the notification block stays email-only by default.
      subscriber_sns_topic_arns = aws_sns_topic.budget[*].arn
    }
  }
}

# ── TTL reaper (ADR-006.4): EventBridge hourly → Lambda deleting expired ────
# ttl-tagged resources. SKELETON: infra wired, function body pending
# (HANDOFF §4 — resource-groups query on tag `ttl` < now, delete by type;
# aws-nuke rejected in favor of this scoped reaper).
resource "aws_iam_role" "reaper" {
  name = "coppice-lab-reaper"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "reaper" {
  name = "reaper"
  role = aws_iam_role.reaper.id
  # Exact delete surface mirroring reaper.py's _DISPATCH (HANDOFF §4). Deletes
  # are constrained by a Null condition requiring the `ttl` resource tag to
  # exist — defence-in-depth behind the function's own "never touch untagged /
  # stack=base" guard. tag:GetResources has no resource-level scoping; logs are
  # scoped to the reaper's own log group.
  # TODO(v5): during EXP-55, confirm every listed action honours the
  # aws:ResourceTag/ttl condition (relax per-action if a service rejects it),
  # then scope Resource by real ARNs once account/region are pinned.
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "TtlTaggedDeletes"
        Effect = "Allow"
        Action = [
          "ec2:TerminateInstances",
          "ec2:DeleteNatGateway",
          "ec2:ReleaseAddress",
          "eks:DeleteCluster",
          "eks:DeleteNodegroup",
          "rds:DeleteDBInstance",
          "elasticloadbalancing:DeleteLoadBalancer",
        ]
        Resource  = "*" # TODO(v5): unknown ARNs pre-account; ttl-tag condition gates it
        Condition = { Null = { "aws:ResourceTag/ttl" = "false" } }
      },
      {
        Sid      = "DiscoverTaggedResources"
        Effect   = "Allow"
        Action   = ["tag:GetResources"]
        Resource = "*" # tag:GetResources does not support resource-level scoping
      },
      {
        Sid    = "ReaperLogs"
        Effect = "Allow"
        Action = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
        # TODO(v5): '*' account segment until the account id is pinned.
        Resource = "arn:aws:logs:${var.aws_region}:*:log-group:/aws/lambda/coppice-lab-reaper:*"
      },
    ]
  })
}

resource "aws_lambda_function" "reaper" {
  function_name = "coppice-lab-reaper"
  role          = aws_iam_role.reaper.arn
  runtime       = "python3.12"
  handler       = "reaper.handler"
  filename      = "${path.module}/reaper/reaper.zip" # built by `make aws-reaper-pack` (HANDOFF §4)
  # Guarded so `terraform validate` (no zip yet) passes; the zip must exist by
  # plan/apply time. fileexists short-circuits before filebase64sha256 runs.
  source_code_hash = fileexists("${path.module}/reaper/reaper.zip") ? filebase64sha256("${path.module}/reaper/reaper.zip") : null
  timeout          = 60
  environment {
    variables = { DRY_RUN = "true" } # flip only after EXP-55 dry-run proof
  }
}

resource "aws_cloudwatch_event_rule" "reaper_hourly" {
  name                = "coppice-lab-reaper-hourly"
  schedule_expression = "rate(1 hour)"
}

resource "aws_cloudwatch_event_target" "reaper" {
  rule = aws_cloudwatch_event_rule.reaper_hourly.name
  arn  = aws_lambda_function.reaper.arn
}

resource "aws_lambda_permission" "reaper_events" {
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.reaper.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.reaper_hourly.arn
}

# ── Budget → ntfy (ADR-006.4, HANDOFF §3) — optional, gated on ntfy_topic_url ─
# Budget breaches publish to this SNS topic (in addition to email); the
# notifier Lambda subscribes and re-POSTs each to ntfy, mirroring the
# scripts/obs/ntfy-relay payload mapping (Title/Priority/Tags). The whole path
# counts to 0 when ntfy_topic_url is "" — email stays the v0 exit channel.
locals {
  ntfy_enabled = var.ntfy_topic_url != "" ? 1 : 0
}

resource "aws_sns_topic" "budget" {
  count = local.ntfy_enabled
  name  = "coppice-lab-budget"
}

# AWS Budgets must be allowed to publish to the topic.
resource "aws_sns_topic_policy" "budget" {
  count = local.ntfy_enabled
  arn   = aws_sns_topic.budget[0].arn
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "AllowBudgetsPublish"
      Effect    = "Allow"
      Principal = { Service = "budgets.amazonaws.com" }
      Action    = "SNS:Publish"
      Resource  = aws_sns_topic.budget[0].arn
    }]
  })
}

resource "aws_iam_role" "ntfy_notifier" {
  count = local.ntfy_enabled
  name  = "coppice-lab-ntfy-notifier"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "ntfy_notifier" {
  count = local.ntfy_enabled
  name  = "ntfy-notifier"
  role  = aws_iam_role.ntfy_notifier[0].id
  # Logs only — the notifier makes no AWS API calls (SNS event carries the msg).
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
      # TODO(v5): '*' account segment until the account id is pinned.
      Resource = "arn:aws:logs:${var.aws_region}:*:log-group:/aws/lambda/coppice-lab-ntfy-notifier:*"
    }]
  })
}

resource "aws_lambda_function" "ntfy_notifier" {
  count         = local.ntfy_enabled
  function_name = "coppice-lab-ntfy-notifier"
  role          = aws_iam_role.ntfy_notifier[0].arn
  runtime       = "python3.12"
  handler       = "notifier.handler"
  filename      = "${path.module}/ntfy-notifier/ntfy-notifier.zip" # `make aws-ntfy-pack`
  # Guarded like the reaper so validate passes before the zip is built.
  source_code_hash = fileexists("${path.module}/ntfy-notifier/ntfy-notifier.zip") ? filebase64sha256("${path.module}/ntfy-notifier/ntfy-notifier.zip") : null
  timeout          = 15
  environment {
    variables = { NTFY_TOPIC_URL = var.ntfy_topic_url }
  }
}

resource "aws_sns_topic_subscription" "ntfy" {
  count     = local.ntfy_enabled
  topic_arn = aws_sns_topic.budget[0].arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.ntfy_notifier[0].arn
}

resource "aws_lambda_permission" "ntfy_sns" {
  count         = local.ntfy_enabled
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.ntfy_notifier[0].function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.budget[0].arn
}

output "ecr_registry" { value = split("/", aws_ecr_repository.svc["api-service"].repository_url)[0] }
output "zone_id" { value = aws_route53_zone.lab.zone_id }
output "zone_ns" { value = aws_route53_zone.lab.name_servers }
