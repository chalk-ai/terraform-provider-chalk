# The role is fixed by the resource type; the referenced cloud storage is
# bound to the env for that role. At most one storage per (environment_id, role).

resource "chalk_environment_source_bundle_cloud_storage_binding" "example" {
  environment_id   = "your-env-id"
  cloud_storage_id = chalk_unmanaged_cloud_storage.source_bundle.id
}
