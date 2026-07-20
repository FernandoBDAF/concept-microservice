# Run ONCE with local state, before anything else (HANDOFF step 1):
#   terraform init && terraform apply -var-file=../terraform.tfvars
# Then base/ and session/ point their S3 backends here. The classic
# chicken-and-egg: this stack's own state stays local (commit nothing).

terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 6.0" }
  }
}

variable "aws_region" { type = string }
variable "aws_profile" { type = string }
variable "aws_account_id" {
  type        = string
  description = "the dedicated lab account id; guards against applying to the wrong account (ADR-006.5)"
}

# lab_domain/budget_limit/alert_email accepted so one tfvars feeds all stacks
variable "lab_domain" {
  type    = string
  default = ""
}
variable "budget_limit" {
  type    = number
  default = 0
}
variable "alert_email" {
  type    = string
  default = ""
}

provider "aws" {
  region  = var.aws_region
  profile = var.aws_profile
  # Wrong-account guard (ADR-006.5): abort unless creds are the lab account.
  allowed_account_ids = [var.aws_account_id]
}

data "aws_caller_identity" "current" {}

resource "aws_s3_bucket" "tfstate" {
  bucket = "coppice-lab-tfstate-${data.aws_caller_identity.current.account_id}"
  # tfstate survives aws-down by design (ADR-006.4 persistent list)
  force_destroy = false
}

resource "aws_s3_bucket_versioning" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  versioning_configuration { status = "Enabled" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  rule {
    apply_server_side_encryption_by_default { sse_algorithm = "aws:kms" }
  }
}

resource "aws_s3_bucket_public_access_block" "tfstate" {
  bucket                  = aws_s3_bucket.tfstate.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_dynamodb_table" "tf_lock" {
  name         = "coppice-lab-tf-lock"
  billing_mode = "PAY_PER_REQUEST" # $0 idle
  hash_key     = "LockID"
  attribute {
    name = "LockID"
    type = "S"
  }
}

output "state_bucket" { value = aws_s3_bucket.tfstate.bucket }
output "lock_table" { value = aws_dynamodb_table.tf_lock.name }
