# Changelog

## 2.0.0

BREAKING CHANGES:

* Remove read-only (computed) attributes from resources across the provider. They added `(known after apply)` noise to every plan; configurations that reference them must stop doing so (wire resources together via `id`). The removed attributes per resource:
  * `resource/chalk_managed_cloud_storage`: `uri`, `managed`, `name`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * `resource/chalk_unmanaged_cloud_storage`: `managed`, `name`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * `resource/chalk_managed_container_registry`: `name`, `kind`, `managed`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * `resource/chalk_unmanaged_container_registry`: `kind`, `managed`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * All cloud storage binding resources (`chalk_cluster_{dataset,plan_stages,source_bundle,model_registry,volume}_cloud_storage_binding`, `chalk_environment_{dataset,plan_stages,source_bundle,model_registry}_cloud_storage_binding`): `created_at`, `updated_at`
  * `resource/chalk_managed_cluster`: `name`, `kind`, `designator`
  * `resource/chalk_managed_aws_vpc`, `resource/chalk_managed_gcp_vpc`: `name`, `designator`
  * `resource/chalk_project`, `resource/chalk_kubernetes_cluster`: `team_id`
  * `resource/chalk_scaling_group`: `status`, `status_message`, `web_url`
  * `resource/chalk_unmanaged_cluster_background_persistence`: `writers[*].name` (the server derives writer names from `bus_subscriber_type`)

`id` is kept on every resource, and `chalk_service_token` keeps `client_id`/`client_secret` (the secret is only returned at creation and cannot be re-fetched). No state migration is needed: removed attributes are dropped from existing state automatically on the next refresh.

See the [v2.0.0 upgrade guide](docs/guides/v2-upgrade-guide.md) for details.

## 1.0.0

BREAKING CHANGES:

* `resource/chalk_managed_environment`, `resource/chalk_unmanaged_environment`: Remove deprecated `environment_buckets` attribute. Use the environment-scoped cloud storage binding resources instead (`chalk_environment_dataset_cloud_storage_binding`, `chalk_environment_plan_stages_cloud_storage_binding`, `chalk_environment_source_bundle_cloud_storage_binding`, `chalk_environment_model_registry_cloud_storage_binding`).
* Remove deprecated `chalk_environment` resource. Use `chalk_managed_environment` or `chalk_unmanaged_environment` instead.
* Remove deprecated `chalk_cluster_background_persistence` resource. Use `chalk_unmanaged_cluster_background_persistence` instead.

See the [v1.0.0 upgrade guide](docs/guides/v1-upgrade-guide.md) for migration steps.
