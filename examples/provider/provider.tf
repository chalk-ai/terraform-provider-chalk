terraform {
  required_providers {
    chalk = {
      source = "chalk-ai/chalk"
    }
  }
}

provider "chalk" {
  # Configuration options
  api_token = var.chalk_api_token
  api_url   = "https://api.chalk.ai" # Optional, defaults to https://api.chalk.ai
}