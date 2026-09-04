# Changelog

## 1.0.5

NOTES:

* `resource/chalk_telemetry`: the telemetry runtime is now pinned to `VECTOR`. It is set on create and included in the field mask on every update, and is deliberately not exposed as an argument. Customer exporters (the `exporters` block) are only rendered on the Vector runtime — on the OTel runtime the server accepts and stores the configuration but never deploys a sink, so an `exporters` block could previously apply cleanly and silently export nothing.

  Three consequences for existing deployments on the OTel runtime:

  * The switch happens on the next apply that changes something else in the resource, since Terraform only calls Update when a tracked attribute differs. Upgrading the provider alone does not move a deployment.
  * Vector's default resource requests are higher than OTel's — the aggregator defaults to 6 CPU / 4Gi rather than 1 CPU / 2Gi. Deployments that do not declare `aggregator_spec` will pick up the larger request; deployments that do declare it keep their configured values, which may be undersized for Vector. The collector default is unchanged at 100m CPU / 128Mi.
  * A deployment whose stored spec sets `otel_collector_image` will fail validation with `otel_collector_image is only supported when telemetry_runtime is OTEL`. Clear that field before upgrading. Declaring `otel_collector_spec` in Terraform also clears it, because the mask replaces the whole message.

## 1.0.4

NOTES:

* Restore the read-only `designator` attribute on `chalk_managed_cloud_storage`, `chalk_unmanaged_cloud_storage`, `chalk_managed_container_registry`, `chalk_unmanaged_container_registry`, `chalk_managed_cluster`, `chalk_managed_aws_vpc`, and `chalk_managed_gcp_vpc`. The server generates it and it cannot be derived, and it appears in managed cluster DNS zones, so it must be readable from Terraform.

## 1.0.2

NOTES:

* Remove read-only (computed) attributes from provider resources. They could not be set and only added `(known after apply)` noise to plans. Existing state is cleaned up automatically on the next refresh; the only configs affected are ones that referenced a removed field (e.g. in an `output`), which must drop the reference. `id` and `chalk_service_token`'s `client_id`/`client_secret` are unchanged. Removed per resource:
  * `chalk_managed_cloud_storage`: `managed`, `name`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * `chalk_unmanaged_cloud_storage`: `managed`, `name`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * `chalk_managed_container_registry`: `kind`, `managed`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * `chalk_unmanaged_container_registry`: `kind`, `managed`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at`
  * all cloud storage binding resources: `created_at`, `updated_at`
  * `chalk_managed_cluster`: `name`, `kind`, `designator`
  * `chalk_managed_aws_vpc`, `chalk_managed_gcp_vpc`: `name`, `designator`
  * `chalk_project`, `chalk_kubernetes_cluster`: `team_id`
  * `chalk_scaling_group`: `status`, `status_message`

## 1.0.0

BREAKING CHANGES:

* `resource/chalk_managed_environment`, `resource/chalk_unmanaged_environment`: Remove deprecated `environment_buckets` attribute. Use the environment-scoped cloud storage binding resources instead (`chalk_environment_dataset_cloud_storage_binding`, `chalk_environment_plan_stages_cloud_storage_binding`, `chalk_environment_source_bundle_cloud_storage_binding`, `chalk_environment_model_registry_cloud_storage_binding`).
* Remove deprecated `chalk_environment` resource. Use `chalk_managed_environment` or `chalk_unmanaged_environment` instead.
* Remove deprecated `chalk_cluster_background_persistence` resource. Use `chalk_unmanaged_cluster_background_persistence` instead.

See the [v1.0.0 upgrade guide](docs/guides/v1-upgrade-guide.md) for migration steps.
