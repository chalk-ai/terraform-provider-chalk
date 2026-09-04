---
subcategory: ""
page_title: "Chalk Provider: Changelog"
description: |-
  Changes to Chalk Terraform resources, data sources, attributes, and required permissions.
---

# Chalk provider changelog

For migration guidance and non-schema changes, see the [project changelog](https://github.com/chalk-ai/terraform-provider-chalk/blob/main/CHANGELOG.md).

## Unreleased

No schema or permission changes.

## v1.0.8

### Resources

- Added `chalk_cluster_host_pool`.
- Added `chalk_environment_host_pool`.
- Removed attribute `chalk_kubernetes_cluster.data_plane_controller.host_pools` (`list(object)`).
- Removed attribute `chalk_kubernetes_cluster.data_plane_controller.host_pools.count` (`number`).
- Removed attribute `chalk_kubernetes_cluster.data_plane_controller.host_pools.cpu` (`string`).
- Removed attribute `chalk_kubernetes_cluster.data_plane_controller.host_pools.machine_family` (`string`).
- Removed attribute `chalk_kubernetes_cluster.data_plane_controller.host_pools.memory` (`string`).
- Removed attribute `chalk_kubernetes_cluster.data_plane_controller.host_pools.name` (`string`).
- Removed attribute `chalk_managed_cluster.data_plane_controller.host_pools` (`list(object)`).
- Removed attribute `chalk_managed_cluster.data_plane_controller.host_pools.count` (`number`).
- Removed attribute `chalk_managed_cluster.data_plane_controller.host_pools.cpu` (`string`).
- Removed attribute `chalk_managed_cluster.data_plane_controller.host_pools.machine_family` (`string`).
- Removed attribute `chalk_managed_cluster.data_plane_controller.host_pools.memory` (`string`).
- Removed attribute `chalk_managed_cluster.data_plane_controller.host_pools.name` (`string`).
- Added attribute `chalk_scaling_group.scaling_spec.shutdown_delay` (`string`).
- Removed attribute `chalk_scaling_group.scaling_spec.shutdown_delay_seconds` (`number`).

## v1.0.7

### Resources

- Added attribute `chalk_telemetry.runtime` (`string`).

## v1.0.6

### Resources

- Added attribute `chalk_cluster_gateway.certificate_issuer_ref` (`object`).
- Added attribute `chalk_cluster_gateway.certificate_issuer_ref.group` (`string`).
- Added attribute `chalk_cluster_gateway.certificate_issuer_ref.kind` (`string`).
- Added attribute `chalk_cluster_gateway.certificate_issuer_ref.name` (`string`).
- Added attribute `chalk_unmanaged_environment.dataplane_db_secret` (`string`).

## v1.0.5

### Resources

- Added `chalk_sandbox`.

## v1.0.4

### Resources

- Added attribute `chalk_managed_aws_vpc.designator` (`string`).
- Added attribute `chalk_managed_cloud_storage.designator` (`string`).
- Added attribute `chalk_managed_cluster.designator` (`string`).
- Added attribute `chalk_managed_container_registry.designator` (`string`).
- Added attribute `chalk_managed_gcp_vpc.designator` (`string`).
- Added attribute `chalk_telemetry.exporters` (`object`).
- Added attribute `chalk_telemetry.exporters.datadog` (`object`).
- Added attribute `chalk_telemetry.exporters.datadog.api_host` (`string`).
- Added attribute `chalk_telemetry.exporters.datadog.api_key_secret_reference` (`string`).
- Added attribute `chalk_telemetry.exporters.datadog.logs` (`object`).
- Added attribute `chalk_telemetry.exporters.datadog.logs.enabled` (`bool`).
- Added attribute `chalk_telemetry.exporters.datadog.metrics` (`object`).
- Added attribute `chalk_telemetry.exporters.datadog.metrics.enabled` (`bool`).
- Added attribute `chalk_telemetry.exporters.datadog.traces` (`object`).
- Added attribute `chalk_telemetry.exporters.datadog.traces.enabled` (`bool`).
- Added attribute `chalk_telemetry.exporters.otlp` (`object`).
- Added attribute `chalk_telemetry.exporters.otlp.authorization_header_secret_reference` (`string`).
- Added attribute `chalk_telemetry.exporters.otlp.enabled` (`bool`).
- Added attribute `chalk_telemetry.exporters.otlp.url` (`string`).
- Added attribute `chalk_unmanaged_cloud_storage.designator` (`string`).
- Added attribute `chalk_unmanaged_container_registry.designator` (`string`).
