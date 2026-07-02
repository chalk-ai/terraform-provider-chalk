# A reference to a cloud container registry plus the credential used to reach it.
# The registry kind is inferred from which config block you set (gar/ecr/acr);
# exactly one must be provided, and its cloud provider must match the credential.
#
# Creating this performs a live access check against the registry using the
# credential, so the credential must exist first.

# Google Artifact Registry
resource "chalk_cloud_container_registry" "gar" {
  name                = "us-central1-docker.pkg.dev/my-project/my-repo"
  cloud_credential_id = chalk_gcp_cloud_credentials.example.id
  config = {
    gar = {
      repository_name = "chalk-images"
    }
  }
}

# AWS Elastic Container Registry
resource "chalk_cloud_container_registry" "ecr" {
  name                = "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo"
  cloud_credential_id = chalk_aws_cloud_credentials.example.id
  config = {
    ecr = {
      repository_name = "chalk-images"
    }
  }
}
