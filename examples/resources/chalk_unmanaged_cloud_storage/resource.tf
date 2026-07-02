# A reference to an existing (unmanaged) bucket plus the credential used to reach
# it. kind is optional — when omitted, Chalk infers it from the credential; when
# set, the uri scheme must match it.
#
# Creating this performs a live Head against the bucket using the credential, so
# the credential (and the bucket's IAM grants) must exist first.

resource "chalk_unmanaged_cloud_storage" "datasets" {
  uri                 = "s3://my-chalk-datasets/prefix"
  cloud_credential_id = chalk_aws_cloud_credentials.example.id
}
