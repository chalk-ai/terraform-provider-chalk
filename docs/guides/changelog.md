---
subcategory: ""
page_title: "Chalk Provider: Changelog"
description: |-
  Changes to Chalk Terraform resources, data sources, attributes, and required permissions.
---

# Chalk provider changelog

For migration guidance and non-schema changes, see the [project changelog](https://github.com/chalk-ai/terraform-provider-chalk/blob/main/CHANGELOG.md).

## Unreleased

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
