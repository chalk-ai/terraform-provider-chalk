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

- Added attribute `chalk_cluster_gateway.certificate_issuer_ref` (`object`).
- Added attribute `chalk_cluster_gateway.certificate_issuer_ref.group` (`string`).
- Added attribute `chalk_cluster_gateway.certificate_issuer_ref.kind` (`string`).
- Added attribute `chalk_cluster_gateway.certificate_issuer_ref.name` (`string`).
- Added attribute `chalk_telemetry.runtime` (`string`).
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
