# A chalk_cloud_storage is a *reference* to an existing bucket plus the cloud
# credential used to reach it. Chalk does not create the bucket.
#
# IMPORTANT ordering: creating this resource performs a live access check against
# the bucket using the referenced credential. The apply fails unless the
# credential can already reach the bucket, so the credential (and the bucket's
# real IAM grants) must exist first. Terraform's dependency on
# cloud_credential_id enforces the credential ordering automatically.

resource "chalk_aws_cloud_credentials" "example" {
  name                    = "example-creds"
  aws_account_id          = "123456789012"
  aws_management_role_arn = "arn:aws:iam::123456789012:role/chalk-management"
  aws_region              = "us-east-1"
}

# S3 bucket reference.
resource "chalk_cloud_storage" "datasets" {
  kind                = "s3"
  uri                 = "s3://my-chalk-datasets/prefix"
  cloud_credential_id = chalk_aws_cloud_credentials.example.id
}

# GCS example.
# resource "chalk_cloud_storage" "gcs_datasets" {
#   kind                = "gcs"
#   uri                 = "gs://my-chalk-datasets/prefix"
#   cloud_credential_id = chalk_gcp_cloud_credentials.example.id
# }

# Azure Blob Storage example (https:// or abfss:// forms are both accepted).
# resource "chalk_cloud_storage" "abs_datasets" {
#   kind                = "abs"
#   uri                 = "https://myaccount.blob.core.windows.net/mycontainer/prefix"
#   cloud_credential_id = chalk_azure_cloud_credentials.example.id
# }
