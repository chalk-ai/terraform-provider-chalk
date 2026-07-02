# The role is fixed by the resource type; the referenced cloud storage is
# bound to the cluster for that role. At most one storage per (cluster_id, role).

resource "chalk_cluster_source_bundle_cloud_storage_binding" "example" {
  cluster_id       = "your-cluster-id"
  cloud_storage_id = chalk_unmanaged_cloud_storage.source_bundle.id
}
