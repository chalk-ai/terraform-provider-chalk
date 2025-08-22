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
  }
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

resource "chalk_kubernetes_cluster" "cluster" {
  kind               = "EKS_STANDARD"
  kubernetes_version = "1.32"
  managed            = false
  name               = "cluster-${random_pet.pet.id}"
}

resource "chalk_environment" "test" {
  name       = "env-${random_pet.pet.id}"
  project_id = chalk_project.test.id
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