# GitHub Actions OIDC deploy identity (ADR-006.7, ADR-010.2 phase 3).
# Lives in the *base* stack: the provider + role idle at $0 and must outlive
# any ephemeral session (the deploy pipeline is meaningless if the trust
# anchor is torn down with the cluster). Consumed by
# .github/workflows/deploy-aws.yml via configure-aws-credentials — no
# long-lived keys ever leave AWS. EXP-54 proves the end-to-end path.
#
# NOTE: kubectl rights are NOT granted here. Assuming this role only yields
# AWS-API access (ECR + eks:DescribeCluster → kubeconfig). Authorizing the
# role inside the cluster requires an EKS *access entry* + access-policy
# association on the SESSION stack (that cluster is ephemeral, so the entry
# belongs there, not in this persistent stack). See wp4.md integration notes;
# the session stack consumes `oidc_deploy_role_arn` (output below).

data "aws_caller_identity" "current" {}

# ── OIDC provider for GitHub Actions ─────────────────────────────────────────
# Current AWS validates the GitHub cert chain against its own trust store and
# effectively IGNORES the thumbprint_list — but the argument is still REQUIRED
# by the API, so we pass the well-known GitHub Actions intermediate/root
# thumbprints. aud = sts.amazonaws.com (configure-aws-credentials default).
resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
  thumbprint_list = [
    "6938fd4d98bab03faadb97b34396831e3780aea1",
    "1c58a3a8518e8759bf075b76b750d4f2df264fcd",
  ]
}

# Subjects allowed to assume the deploy role. Default is NARROW: main branch
# pushes and any tag (the on-tag deploy trigger). Widen ONLY during bring-up
# (e.g. add "repo:FernandoBDAF/coppice:*" while debugging EXP-54) then revert.
variable "deploy_oidc_subjects" {
  type        = list(string)
  description = "GitHub OIDC `sub` claims permitted to assume coppice-lab-deploy (StringLike; widen only during bring-up)."
  default = [
    "repo:FernandoBDAF/coppice:ref:refs/heads/main",
    "repo:FernandoBDAF/coppice:ref:refs/tags/*",
  ]
}

# ── Deploy role — assumed by GitHub Actions via OIDC ─────────────────────────
resource "aws_iam_role" "deploy" {
  name = "coppice-lab-deploy"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Federated = aws_iam_openid_connect_provider.github.arn }
      Action    = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
        }
        StringLike = {
          "token.actions.githubusercontent.com:sub" = var.deploy_oidc_subjects
        }
      }
    }]
  })
}

# ECR: an account-wide GetAuthorizationToken (login) + repo-scoped push/pull
# on exactly the coppice-lab/* repos declared in main.tf (referenced, so the
# ARN set stays in lockstep with the ECR repos).
resource "aws_iam_role_policy" "deploy_ecr" {
  name = "ecr-push"
  role = aws_iam_role.deploy.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "EcrLogin"
        Effect   = "Allow"
        Action   = "ecr:GetAuthorizationToken"
        Resource = "*"
      },
      {
        Sid    = "EcrPushPull"
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
          "ecr:InitiateLayerUpload",
          "ecr:UploadLayerPart",
          "ecr:CompleteLayerUpload",
          "ecr:PutImage",
        ]
        Resource = [for r in aws_ecr_repository.svc : r.arn]
      },
    ]
  })
}

# EKS: DescribeCluster is all the AWS-API surface the pipeline needs — it lets
# `aws eks update-kubeconfig` mint a kubeconfig. In-cluster (kubectl) authz is
# granted separately via a session-stack access entry (see header note).
resource "aws_iam_role_policy" "deploy_eks" {
  name = "eks-describe"
  role = aws_iam_role.deploy.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid    = "EksDescribe"
      Effect = "Allow"
      # DescribeCluster is all update-kubeconfig needs; ListClusters is omitted
      # (it requires Resource="*", which this scoped ARN deliberately withholds).
      Action   = "eks:DescribeCluster"
      Resource = "arn:aws:eks:${var.aws_region}:${data.aws_caller_identity.current.account_id}:cluster/coppice-lab"
    }]
  })
}

# Consumed by: (1) the GitHub repo variable AWS_DEPLOY_ROLE_ARN, (2) the
# session stack's aws_eks_access_entry principal_arn (wp4.md notes).
output "oidc_deploy_role_arn" {
  value       = aws_iam_role.deploy.arn
  description = "ARN of coppice-lab-deploy — set as GitHub repo var AWS_DEPLOY_ROLE_ARN and grant an EKS access entry on the session cluster."
}
