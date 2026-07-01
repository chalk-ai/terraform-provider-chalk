# Bind a cloud storage to a cluster for a specific role. A cluster has at most
# one storage per role; the binding is keyed by (cluster_id, storage_role), not
# by cloud_storage_id.

resource "chalk_cluster_cloud_storage_binding" "plan_stages" {
  cluster_id       = "your-cluster-id"
  cloud_storage_id = chalk_cloud_storage.datasets.id
  storage_role     = "PLAN_STAGES"
}

# Valid storage_role values: DATASET, PLAN_STAGES, SOURCE_BUNDLE, MODEL_REGISTRY, VOLUME.
