terraform {
  required_providers {
    chalk = {
      source = "chalk-ai/chalk"
    }
  }
}

provider "chalk" {
  # Authentication option 1: client ID and secret
  client_id     = var.chalk_client_id
  client_secret = var.chalk_client_secret

  # Authentication option 2: JWT token directly
  # jwt = var.chalk_jwt

  # Authentication option 3: command to retrieve JWT token (e.g. Chalk CLI)
  # jwt_command_process = "chalk token"

  # Optional: override the API server (defaults to https://api.chalk.ai)
  # api_server = "https://api.chalk.ai"
}