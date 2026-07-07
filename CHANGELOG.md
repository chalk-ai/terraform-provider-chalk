# Changelog

## 1.0.0

BREAKING CHANGES:

* `resource/chalk_managed_environment`, `resource/chalk_unmanaged_environment`: Remove deprecated `environment_buckets` attribute. Use the environment-scoped cloud storage binding resources instead (`chalk_environment_dataset_cloud_storage_binding`, `chalk_environment_plan_stages_cloud_storage_binding`, `chalk_environment_source_bundle_cloud_storage_binding`, `chalk_environment_model_registry_cloud_storage_binding`).
* Remove deprecated `chalk_environment` resource. Use `chalk_managed_environment` or `chalk_unmanaged_environment` instead.
* Remove deprecated `chalk_cluster_background_persistence` resource. Use `chalk_unmanaged_cluster_background_persistence` instead.

See the [v1.0.0 upgrade guide](docs/guides/v1-upgrade-guide.md) for migration steps.
