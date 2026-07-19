# Kubernetes + Helm providers for the in-cluster addons (addons.tf). Built
# from the EKS module outputs; auth is exec-based via `aws eks get-token` so no
# kubeconfig or long-lived token is stored in state (aws CLI is present at
# apply time — `make aws-up` runs on a box that has it).
#
# The kubernetes/helm/random providers are declared in main.tf's
# required_providers block (Terraform permits only one such block per module).

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}

provider "kubernetes" {
  host                   = module.eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", module.eks.cluster_name, "--region", var.aws_region]
  }
}

provider "helm" {
  kubernetes {
    host                   = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args        = ["eks", "get-token", "--cluster-name", module.eks.cluster_name, "--region", var.aws_region]
    }
  }
}
