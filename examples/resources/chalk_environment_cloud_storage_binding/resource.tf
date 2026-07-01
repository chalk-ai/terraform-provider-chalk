# Bind a cloud storage to an environment for a specific role. An environment has
# at most one storage per role; the binding is keyed by (environment_id,
# storage_role), not by cloud_storage_id.

resource "chalk_environment_cloud_storage_binding" "datasets" {
  environment_id   = "your-environment-id"
  cloud_storage_id = chalk_cloud_storage.datasets.id
  storage_role     = "DATASET"
}

# Valid storage_role values: DATASET, PLAN_STAGES, SOURCE_BUNDLE, MODEL_REGISTRY, VOLUME.
