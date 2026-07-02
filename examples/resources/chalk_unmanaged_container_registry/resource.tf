# A reference to an existing (unmanaged) container registry plus the credential
# used to reach it. The registry kind (GAR/ECR/ACR) is derived by the server from
# name, so the credential's cloud provider must match it.
#
# Creating this performs a live access check against the registry using the
# credential, so the credential must exist first.

resource "chalk_unmanaged_container_registry" "gar" {
  name                = "us-docker.pkg.dev/my-project/repo-for-compute"
  cloud_credential_id = chalk_gcp_cloud_credentials.example.id
}
