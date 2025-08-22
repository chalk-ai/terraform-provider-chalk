# main.tf
terraform {
  required_providers {
    chalk = {
      source  = "registry.terraform.io/chalk-ai/chalk"
      version = "0.1.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.5.1"
    }
    google = {
      source  = "hashicorp/google"
      version = "5.0.0"
    }
  }
}
data "google_client_openid_userinfo" "me" {}

locals {
  sanitized_email = replace(data.google_client_openid_userinfo.me.email, "/[^a-zA-Z0-9-]/", "-")
}

# Fixed token
provider "chalk" {
  client_id     = "token-environment-fixed"
  client_secret = "ts-d2c87cbb1dd742c666d547d393a5341e011683206891fcc6dc2780ffd5cdf67e"
  api_server    = "http://localhost:8080"
}

resource "chalk_project" "test" {
  name = "project-${random_pet.pet.id}"
}

resource "random_pet" "pet" {}

resource "chalk_cloud_credentials" "creds" {
  kind                    = "aws"
  name                    = "creds-${random_pet.pet.id}"
  aws_account_id          = "009160067517"
  aws_management_role_arn = "arn:aws:iam::009160067517:role/chalk-cicd-test-Chalk-Api-Management"
  aws_region              = "us-east-1"

  gcp_workload_identity {
    pool_id         = "cicd-009160067517-pool"
    provider_id     = "cicd-009160067517-provider"
    service_account = "aws-workload-009160067517@chalk-infra.iam.gserviceaccount.com"
    project_number  = "610611181724"
  }

  docker_build_config {
    builder            = "argo"
    notification_topic = "arn:aws:sqs:us-east-1:009160067517:argo-build-queue-${local.sanitized_email}"
  }
}

resource "chalk_kubernetes_cluster" "cluster" {
  kind                = "EKS_STANDARD"
  kubernetes_version  = "1.32"
  managed             = false
  name                = "chalk-cicd-test-eks-cluster"
  cloud_credential_id = chalk_cloud_credentials.creds.id
}

resource "chalk_environment" "test" {
  name            = "env-${random_pet.pet.id}"
  project_id      = chalk_project.test.id
  kube_cluster_id = chalk_kubernetes_cluster.cluster.id
}

output "env_info" {
  value = chalk_environment.test.name
}

output "project_info" {
  value = chalk_project.test.name
}

output "cluster_info" {
  value = chalk_kubernetes_cluster.cluster.name
}

output "creds_info" {
  value = chalk_cloud_credentials.creds.name
}