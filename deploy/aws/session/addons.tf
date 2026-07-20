# In-cluster addons (HANDOFF §5.2) — installed via helm_release on the EKS
# cluster once it exists. Each gets its own IRSA role (community-standard
# policy from terraform-aws-modules/iam) scoped as tightly as the shared
# module allows; TODO(v5) markers flag the spots broader than ideal.
#
# Chart pins (recorded from `helm search repo` at implementation time,
# 2026-07-19):
#   aws-load-balancer-controller  chart 3.4.2   (app v3.4.2)
#   external-secrets              chart 2.8.0   (app v2.8.0)
#   external-dns                  chart 1.21.1  (app 0.21.0)

# ── aws-load-balancer-controller (ALB ingress, ADR-006.6) ────────────────────
module "irsa_lb_controller" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.0"

  role_name                              = "coppice-lab-alb-controller"
  attach_load_balancer_controller_policy = true

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:aws-load-balancer-controller"]
    }
  }
}

resource "helm_release" "aws_load_balancer_controller" {
  name       = "aws-load-balancer-controller"
  repository = "https://aws.github.io/eks-charts"
  chart      = "aws-load-balancer-controller"
  version    = "3.4.2"
  namespace  = "kube-system"

  set {
    name  = "clusterName"
    value = module.eks.cluster_name
  }
  set {
    name  = "serviceAccount.create"
    value = "true"
  }
  set {
    name  = "serviceAccount.name"
    value = "aws-load-balancer-controller"
  }
  set {
    name  = "serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn"
    value = module.irsa_lb_controller.iam_role_arn
  }
  set {
    name  = "region"
    value = var.aws_region
  }
  set {
    name  = "vpcId"
    value = module.vpc.vpc_id
  }

  # Tag controller-created ALBs/SGs with the SAME keys the session provider's
  # default_tags apply — Terraform-managed resources inherit those tags, but the
  # ALBs/SGs the controller creates out-of-band do NOT, so without this they
  # carry no project/ttl tags and stay invisible to the reaper + assert-clean.sh.
  # ttl reuses the same time_static expression as the provider default_tags.
  set {
    name  = "defaultTags.project"
    value = "coppice-lab"
  }
  set {
    name  = "defaultTags.stack"
    value = "session"
  }
  set {
    name  = "defaultTags.ttl"
    type  = "string" # keep the numeric epoch a string tag, not a helm-coerced number
    value = tostring(time_static.session_start.unix + var.session_ttl_hours * 3600)
  }

  depends_on = [module.eks]
}

# ── external-secrets (Secrets Manager → k8s Secrets, ADR-009.3) ──────────────
module "irsa_external_secrets" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.0"

  role_name                      = "coppice-lab-external-secrets"
  attach_external_secrets_policy = true

  # Scoped to THIS session's secrets. TODO(v5): if the overlay moves the other
  # self-hosted creds (rabbitmq/mongo/jwt) into Secrets Manager too, add their
  # ARNs here or the ExternalSecrets for them will get AccessDenied.
  external_secrets_secrets_manager_arns = [
    "${aws_secretsmanager_secret.postgres_credentials.arn}*",
  ]

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["external-secrets:external-secrets"]
    }
  }
}

resource "helm_release" "external_secrets" {
  name             = "external-secrets"
  repository       = "https://charts.external-secrets.io"
  chart            = "external-secrets"
  version          = "2.8.0"
  namespace        = "external-secrets"
  create_namespace = true

  set {
    name  = "installCRDs"
    value = "true"
  }
  set {
    name  = "serviceAccount.name"
    value = "external-secrets"
  }
  set {
    name  = "serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn"
    value = module.irsa_external_secrets.iam_role_arn
  }

  depends_on = [module.eks]
}

# ── external-dns (Ingress hostnames → Route53, ADR-006.6) ────────────────────
module "irsa_external_dns" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.0"

  role_name                  = "coppice-lab-external-dns"
  attach_external_dns_policy = true
  # Scoped to the lab zone only (least privilege — not account-wide route53).
  external_dns_hosted_zone_arns = [
    "arn:${data.aws_partition.current.partition}:route53:::hostedzone/${data.aws_route53_zone.lab.zone_id}",
  ]

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["external-dns:external-dns"]
    }
  }
}

resource "helm_release" "external_dns" {
  name             = "external-dns"
  repository       = "https://kubernetes-sigs.github.io/external-dns"
  chart            = "external-dns"
  version          = "1.21.1"
  namespace        = "external-dns"
  create_namespace = true

  set {
    name  = "provider.name"
    value = "aws"
  }
  set {
    name  = "policy"
    value = "sync" # create AND delete records so aws-down leaves no orphan RRsets
  }
  set {
    name  = "txtOwnerId"
    value = "coppice-lab"
  }
  set {
    name  = "domainFilters[0]"
    value = var.lab_domain
  }
  set {
    name  = "serviceAccount.name"
    value = "external-dns"
  }
  set {
    name  = "serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn"
    value = module.irsa_external_dns.iam_role_arn
  }

  depends_on = [module.eks]
}

# ── aws-ebs-csi-driver (EKS addon) ───────────────────────────────────────────
# BLOCKER without it: the aws overlay's default StorageClass provisions via
# ebs.csi.aws.com, and EKS ships NO EBS CSI driver by default — every PVC would
# stay Pending. Installed as a managed EKS addon with its own IRSA role (same
# terraform-aws-modules/iam pattern as the controllers above). Kept as a
# standalone aws_eks_addon (not in the eks module's `addons` block) so the addon
# can consume this role's ARN without a module-level dependency cycle.
module "irsa_ebs_csi" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.0"

  role_name             = "coppice-lab-ebs-csi"
  attach_ebs_csi_policy = true # AWS managed service-role/AmazonEBSCSIDriverPolicy

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:ebs-csi-controller-sa"]
    }
  }
}

resource "aws_eks_addon" "ebs_csi" {
  cluster_name             = module.eks.cluster_name
  addon_name               = "aws-ebs-csi-driver"
  service_account_role_arn = module.irsa_ebs_csi.iam_role_arn

  # Node group must exist so the controller pods can schedule.
  depends_on = [module.eks]
}
