# A Chalk-managed cloud storage: Chalk owns the bucket and derives its uri, so you
# supply only the cloud credential. kind is optional — when omitted, Chalk infers
# it from the credential.
#
# Creating this performs a live access check using the credential, so the
# credential (and the bucket's IAM grants) must exist first.

resource "chalk_managed_cloud_storage" "datasets" {
  cloud_credential_id = chalk_aws_cloud_credentials.example.id
}
