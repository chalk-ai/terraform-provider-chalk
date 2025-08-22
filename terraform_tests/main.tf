# main.tf
terraform {
  required_providers {
    chalk = {
      source  = "registry.terraform.io/chalk-ai/chalk"
      version = "0.1.0"
    }
  }
}

# provider "chalk" {
#   client_id = "client-9314816c664be6cb04678e5e3d0e50e2"
#   client_secret = "secret-51f3ce3d4f701f23afdd622015732edf1eb62d0d678b3511b615f9b1f00b4a7d"
#   api_server = "https://api.staging.chalk.ai"
# }

# Local user
# provider "chalk" {
#   client_id = "client-c5b963c72a8c24e6ae81d1d74de46b89"
#   client_secret = "secret-c2cefd5b0f3568f7ce63d09512af39f7ed34946598e3f31984cee3214ea5138b"
#   api_server = "http://localhost:4002"
# }

# Fixed token
provider "chalk" {
  client_id     = "token-environment-fixed"
  client_secret = "ts-d2c87cbb1dd742c666d547d393a5341e011683206891fcc6dc2780ffd5cdf67e"
  api_server    = "http://localhost:8080"
}

# data "chalk_environment" "test3" {
#   id = "cme0jzqp20001kj9k9jxfy80h"
# }

resource "chalk_project" "test" {
  name = "Test Project 5"
  # description = "This is a test project"
}

resource "chalk_environment" "test2" {
  name       = "Test Environment 10"
  project_id = chalk_project.test.id
  # is_default = true
}

# data "chalk_environment" "test" {
#   id = "environment"
# }

output "env_info" {
  value     = chalk_environment.test2
  sensitive = true
}