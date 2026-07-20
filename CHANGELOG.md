# Changelog

## Unreleased

BREAKING CHANGES:

* Remove read-only (computed) attributes from provider resources. They could not be set and only added `(known after apply)` noise to plans. Existing state is cleaned up automatically on the next refresh; the only configs affected are ones that referenced a removed field (e.g. in an `output`), which must drop the reference. `id` and `chalk_service_token`'s `client_id`/`client_secret` are unchanged. Removed per resource:
  * `chalk_managed_cloud_storage`: `uri`, `managed`, `name`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * `chalk_unmanaged_cloud_storage`: `managed`, `name`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * `chalk_managed_container_registry`: `name`, `kind`, `managed`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * `chalk_unmanaged_container_registry`: `kind`, `managed`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * all cloud storage binding resources: `created_at`, `updated_at`
  * `chalk_managed_cluster`: `name`, `kind`, `designator`
  * `chalk_managed_aws_vpc`, `chalk_managed_gcp_vpc`: `name`, `designator`
  * `chalk_project`, `chalk_kubernetes_cluster`: `team_id`
  * `chalk_scaling_group`: `status`, `status_message`, `web_url`

## 1.0.0

BREAKING CHANGES:

* `resource/chalk_managed_environment`, `resource/chalk_unmanaged_environment`: Remove deprecated `environment_buckets` attribute. Use the environment-scoped cloud storage binding resources instead (`chalk_environment_dataset_cloud_storage_binding`, `chalk_environment_plan_stages_cloud_storage_binding`, `chalk_environment_source_bundle_cloud_storage_binding`, `chalk_environment_model_registry_cloud_storage_binding`).
* Remove deprecated `chalk_environment` resource. Use `chalk_managed_environment` or `chalk_unmanaged_environment` instead.
* Remove deprecated `chalk_cluster_background_persistence` resource. Use `chalk_unmanaged_cluster_background_persistence` instead.

See the [v1.0.0 upgrade guide](docs/guides/v1-upgrade-guide.md) for migration steps.
