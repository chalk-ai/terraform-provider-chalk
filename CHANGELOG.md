# Changelog

## Unreleased

* Add optional `chalk_workload_identity` to `chalk_sandbox`: run the sandbox as its own workload principal (`mode = "workload_principal"`) and declare platform-managed `oidc_token_files`, e.g. for GCP workload identity federation without a key in the sandbox. `oidc_token_files` is a set (order does not matter) and requires host compute, so it is rejected at plan time with `runtime = "kube"`. Changing it replaces the sandbox. Requires a Chalk API server that supports sandbox workload identity.

## 1.3.0

* Add `chalk_sandbox_exec` to run one-shot commands in an existing running sandbox. It requires an environment, validates Go duration timeouts at plan time, keeps write-only `stdin` out of state, retains the last 16 KiB of output, and re-runs when `triggers` change.
* Move `chalk_sandbox` to the v2 sandbox API, add opt-in in-place suspend and restore with `desired_state`, preserve unmanaged lifecycle state and server-generated names as null, and make snapshot cleanup on destroy best-effort. Update `chalk-go` to v1.3.30. This requires a Chalk API server that supports the v2 sandbox and sandbox snapshot services.
* `runtime` no longer has a provider-side default; an unset runtime uses the server default (host compute). Set `runtime = "kube"` to keep a Kubernetes pod; note `kube` + `network_policy` is rejected by the server.

## 1.2.0

* Add optional `victoria_metrics` sizing to `chalk_telemetry`: `storage_size`, `storage_replicas`, `insert_replicas`, and `auth_replicas`. Updates preserve unconfigured VictoriaMetrics settings; removing configuration leaves live sizing unchanged.
* Preserve computed telemetry state when an update only relinquishes ownership of optional settings.

## 1.1.1

* Update managed AWS and GCP VPC configurations in place using field masks, including GCP subnet secondary IP ranges. Wait for asynchronous updates to finish and preserve the applied configuration when an update fails.
* Update `chalk-go` and its generated protobuf module to `v1.3.29`. VPC updates require a Chalk API server supporting `UpdateCloudComponentVpc`.

## 1.1.0

* Add optional `machine_size` (`micro`, `small`, `medium`, or `large`) to `chalk_cluster_gateway`, including inferred Envoy resource requests and generation-constrained AWS/GCP placement. Conflicts with `nodepool`; omitted values retain default placement.
* Update `chalk-go` and its generated protobuf module to `v1.3.28`.

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
