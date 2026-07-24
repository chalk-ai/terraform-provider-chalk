---
subcategory: ""
page_title: "Chalk Provider: Changelog"
description: |-
  Machine-generated changes to Chalk Terraform resources, data sources, attributes, and required permissions.
---

# Chalk provider changelog

This changelog is generated from the provider's Terraform schemas and required permissions.
For implementation changes and bug fixes, see the [GitHub release notes](https://github.com/chalk-ai/terraform-provider-chalk/releases).

## Unreleased

No schema or permission changes.

## v1.0.2

### Resources

- Removed computed string attribute from `chalk_cluster_dataset_cloud_storage_binding.created_at`.
- Removed computed string attribute from `chalk_cluster_dataset_cloud_storage_binding.updated_at`.
- Removed computed string attribute from `chalk_cluster_model_registry_cloud_storage_binding.created_at`.
- Removed computed string attribute from `chalk_cluster_model_registry_cloud_storage_binding.updated_at`.
- Removed computed string attribute from `chalk_cluster_plan_stages_cloud_storage_binding.created_at`.
- Removed computed string attribute from `chalk_cluster_plan_stages_cloud_storage_binding.updated_at`.
- Removed computed string attribute from `chalk_cluster_source_bundle_cloud_storage_binding.created_at`.
- Removed computed string attribute from `chalk_cluster_source_bundle_cloud_storage_binding.updated_at`.
- Removed computed string attribute from `chalk_cluster_volume_cloud_storage_binding.created_at`.
- Removed computed string attribute from `chalk_cluster_volume_cloud_storage_binding.updated_at`.
- Removed computed string attribute from `chalk_environment_dataset_cloud_storage_binding.created_at`.
- Removed computed string attribute from `chalk_environment_dataset_cloud_storage_binding.updated_at`.
- Removed computed string attribute from `chalk_environment_model_registry_cloud_storage_binding.created_at`.
- Removed computed string attribute from `chalk_environment_model_registry_cloud_storage_binding.updated_at`.
- Removed computed string attribute from `chalk_environment_plan_stages_cloud_storage_binding.created_at`.
- Removed computed string attribute from `chalk_environment_plan_stages_cloud_storage_binding.updated_at`.
- Removed computed string attribute from `chalk_environment_source_bundle_cloud_storage_binding.created_at`.
- Removed computed string attribute from `chalk_environment_source_bundle_cloud_storage_binding.updated_at`.
- Removed computed string attribute from `chalk_kubernetes_cluster.team_id`.
- Removed computed string attribute from `chalk_managed_aws_vpc.designator`.
- Removed computed string attribute from `chalk_managed_aws_vpc.name`.
- Removed computed string attribute from `chalk_managed_cloud_storage.applied_at`.
- Removed computed string attribute from `chalk_managed_cloud_storage.created_at`.
- Removed computed string attribute from `chalk_managed_cloud_storage.designator`.
- Removed computed bool attribute from `chalk_managed_cloud_storage.managed`.
- Removed computed string attribute from `chalk_managed_cloud_storage.name`.
- Removed computed string attribute from `chalk_managed_cloud_storage.team_id`.
- Removed computed string attribute from `chalk_managed_cloud_storage.updated_at`.
- Removed computed string attribute from `chalk_managed_cluster.designator`.
- Removed computed string attribute from `chalk_managed_cluster.kind`.
- Removed computed string attribute from `chalk_managed_cluster.name`.
- Removed computed string attribute from `chalk_managed_container_registry.applied_at`.
- Removed computed string attribute from `chalk_managed_container_registry.created_at`.
- Removed computed string attribute from `chalk_managed_container_registry.designator`.
- Removed computed string attribute from `chalk_managed_container_registry.kind`.
- Removed computed bool attribute from `chalk_managed_container_registry.managed`.
- Removed computed string attribute from `chalk_managed_container_registry.team_id`.
- Removed computed string attribute from `chalk_managed_container_registry.updated_at`.
- Removed computed string attribute from `chalk_managed_gcp_vpc.designator`.
- Removed computed string attribute from `chalk_managed_gcp_vpc.name`.
- Removed computed string attribute from `chalk_project.team_id`.
- Removed computed string attribute from `chalk_scaling_group.status`.
- Removed computed string attribute from `chalk_scaling_group.status_message`.
- Removed computed string attribute from `chalk_unmanaged_cloud_storage.applied_at`.
- Removed computed string attribute from `chalk_unmanaged_cloud_storage.created_at`.
- Removed computed string attribute from `chalk_unmanaged_cloud_storage.designator`.
- Removed computed bool attribute from `chalk_unmanaged_cloud_storage.managed`.
- Removed computed string attribute from `chalk_unmanaged_cloud_storage.name`.
- Removed computed string attribute from `chalk_unmanaged_cloud_storage.team_id`.
- Removed computed string attribute from `chalk_unmanaged_cloud_storage.updated_at`.
- Removed computed string attribute from `chalk_unmanaged_container_registry.applied_at`.
- Removed computed string attribute from `chalk_unmanaged_container_registry.created_at`.
- Removed computed string attribute from `chalk_unmanaged_container_registry.designator`.
- Removed computed string attribute from `chalk_unmanaged_container_registry.kind`.
- Removed computed bool attribute from `chalk_unmanaged_container_registry.managed`.
- Removed computed string attribute from `chalk_unmanaged_container_registry.team_id`.
- Removed computed string attribute from `chalk_unmanaged_container_registry.updated_at`.

## v1.0.1

### Resources

- Added optional single nested attribute to `chalk_kubernetes_cluster.data_plane_controller`.
- Added optional single nested attribute to `chalk_kubernetes_cluster.data_plane_redis`.
- Added optional single nested attribute to `chalk_kubernetes_cluster.maintenance_window`.
- Added optional single nested attribute to `chalk_managed_cluster.data_plane_controller`.
- Added optional single nested attribute to `chalk_managed_cluster.data_plane_redis`.
- Added optional single nested attribute to `chalk_managed_cluster.maintenance_window`.

## v1.0.0

### Resources

- Removed `chalk_cluster_background_persistence`.
- Removed `chalk_environment`.
- Removed optional single nested attribute from `chalk_managed_environment.environment_buckets`.
- Removed optional single nested attribute from `chalk_unmanaged_environment.environment_buckets`.

## v0.9.28

### Resources

- Added optional string attribute to `chalk_aws_cloud_credentials.aws_permissions_boundary_arn`.

## v0.9.27

### Resources

- Added `chalk_cluster_container_registry_binding`.
- Added `chalk_cluster_dataset_cloud_storage_binding`.
- Added `chalk_cluster_model_registry_cloud_storage_binding`.
- Added `chalk_cluster_plan_stages_cloud_storage_binding`.
- Added `chalk_cluster_source_bundle_cloud_storage_binding`.
- Changed `chalk_cluster_timescale.dns_hostname` computed from false to true.
- Added `chalk_cluster_volume_cloud_storage_binding`.
- Added `chalk_environment_dataset_cloud_storage_binding`.
- Added `chalk_environment_model_registry_cloud_storage_binding`.
- Added `chalk_environment_plan_stages_cloud_storage_binding`.
- Added `chalk_environment_source_bundle_cloud_storage_binding`.
- Added `chalk_managed_cloud_storage`.
- Added `chalk_managed_container_registry`.
- Added optional list nested attribute to `chalk_managed_gcp_vpc.backup_subnets`.
- Added required string attribute to `chalk_managed_gcp_vpc.cloud_credential_id`.
- Added computed string attribute to `chalk_managed_gcp_vpc.designator`.
- Added computed string attribute to `chalk_managed_gcp_vpc.id`.
- Added computed string attribute to `chalk_managed_gcp_vpc.name`.
- Added required list nested attribute to `chalk_managed_gcp_vpc.subnets`.
- Added optional string attribute to `chalk_managed_gcp_vpc.vpc_peer_addr`.
- Added `chalk_unmanaged_cloud_storage`.
- Added `chalk_unmanaged_container_registry`.

### Required permissions

- Resource `chalk_cluster_background_persistence_deployment_binding` changed `project.create` from not team-scoped to team-scoped.
- Resource `chalk_cluster_gateway_binding` changed `project.create` from not team-scoped to team-scoped.
- Resource `chalk_environment_background_persistence_deployment_binding` changed `project.create` from not team-scoped to team-scoped.
- Resource `chalk_environment_gateway_binding` changed `project.create` from not team-scoped to team-scoped.
- Resource `chalk_kubernetes_cluster` changed `project.create` from not team-scoped to team-scoped.
- Resource `chalk_managed_aws_vpc` changed `project.create` from not team-scoped to team-scoped.
- Resource `chalk_managed_aws_vpc` changed `team.admin` from not team-scoped to team-scoped.
- Resource `chalk_managed_cluster` changed `project.create` from not team-scoped to team-scoped.
- Resource `chalk_managed_gcp_vpc` now requires `project.create` (team-scoped).
- Resource `chalk_managed_gcp_vpc` now requires `team.admin` (team-scoped).
- Resource `chalk_private_gateway_binding` changed `project.create` from not team-scoped to team-scoped.
- Resource `chalk_telemetry_binding` changed `project.create` from not team-scoped to team-scoped.

## v0.9.26

### Resources

- Changed `chalk_environment.feature_store_secret` sensitive from true to false.
- Changed `chalk_environment.online_store_secret` sensitive from true to false.
- Changed `chalk_managed_environment.online_store_secret` sensitive from true to false.
- Changed `chalk_unmanaged_environment.online_store_secret` sensitive from true to false.

## v0.9.25

### Resources

- Added optional string attribute to `chalk_cluster_background_persistence.writers.nodepool`.
- Added optional string attribute to `chalk_unmanaged_cluster_background_persistence.autodiscover_key`.
- Added optional string attribute to `chalk_unmanaged_cluster_background_persistence.writers.nodepool`.

## v0.9.24

### Resources

- Changed `chalk_cluster_background_persistence.writers.default_replica_count` computed from true to false.
- Changed `chalk_cluster_background_persistence.writers.gke_spot` computed from true to false.
- Changed `chalk_cluster_background_persistence.writers.hpa_specs.hpa_max_replicas` computed from true to false.
- Changed `chalk_cluster_background_persistence.writers.hpa_specs.hpa_min_replicas` computed from true to false.
- Changed `chalk_cluster_background_persistence.writers.hpa_specs.hpa_target_average_value` computed from true to false.
- Changed `chalk_cluster_background_persistence.writers.load_writer_configmap` computed from true to false.
- Changed `chalk_cluster_background_persistence.writers.query_table_write_drop_ratio` computed from true to false.
- Changed `chalk_cluster_background_persistence.writers.results_writer_skip_producing_feature_metrics` computed from true to false.
- Changed `chalk_unmanaged_cluster_background_persistence.writers.default_replica_count` computed from true to false.
- Changed `chalk_unmanaged_cluster_background_persistence.writers.gke_spot` computed from true to false.
- Changed `chalk_unmanaged_cluster_background_persistence.writers.hpa_specs.hpa_max_replicas` computed from true to false.
- Changed `chalk_unmanaged_cluster_background_persistence.writers.hpa_specs.hpa_min_replicas` computed from true to false.
- Changed `chalk_unmanaged_cluster_background_persistence.writers.hpa_specs.hpa_target_average_value` computed from true to false.
- Changed `chalk_unmanaged_cluster_background_persistence.writers.load_writer_configmap` computed from true to false.
- Changed `chalk_unmanaged_cluster_background_persistence.writers.query_table_write_drop_ratio` computed from true to false.
- Changed `chalk_unmanaged_cluster_background_persistence.writers.results_writer_skip_producing_feature_metrics` computed from true to false.

## v0.9.23

### Resources

- Added optional map(string) attribute to `chalk_cluster_background_persistence.writers.additional_env_vars`.
- Added optional map(string) attribute to `chalk_unmanaged_cluster_background_persistence.writers.additional_env_vars`.

## v0.9.22

### Resources

- Added `chalk_scaling_group`.

## v0.9.21

### Resources

- Added optional string attribute to `chalk_offline_store_connection.snowflake.storage_integration_name`.

## v0.9.20

### Resources

- Added optional string attribute to `chalk_cluster_gateway.load_balancer_class`.

## v0.9.19

### Resources

- Added optional list(string) attribute to `chalk_cluster_timescale.ip_allowlist`.

## v0.9.18

Baseline snapshot. Schema and permission changes from earlier releases are not included.
