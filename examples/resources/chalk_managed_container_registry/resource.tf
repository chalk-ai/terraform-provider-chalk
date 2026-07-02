# A Chalk-managed container registry: Chalk owns the registry and derives its
# name, so you only supply the cloud credential.
#
# Creating this performs a live access check using the credential, so the
# credential must exist first.

resource "chalk_managed_container_registry" "ecr" {
  cloud_credential_id = chalk_aws_cloud_credentials.example.id
}
