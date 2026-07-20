---
subcategory: ""
page_title: "Chalk Provider: Upgrade Guide for Version 2.0.0"
description: |-
  This guide covers the breaking changes introduced in v2.0.0 of the Chalk provider and what you need to do to upgrade.
---

# Upgrading to v2.0.0 of the Chalk provider

Version 2.0.0 removes the read-only (computed) attributes that resources used to
export — server-side metadata such as `created_at`, `team_id`, and `designator`.
These attributes added a `(known after apply)` line to every plan for every resource
instance without carrying configuration meaning. Resources now expose only their
inputs plus `id`.

No state migration is needed: removed attributes are dropped from existing state
automatically the next time a resource is refreshed. Upgrading only requires a
configuration change if you reference one of the removed attributes (for example in
an `output` or another resource's argument) — `terraform plan` will fail with
`Unsupported attribute` at each such reference until it is removed.

## Removed attributes by resource

| Resource | Removed attributes |
|---|---|
| `chalk_managed_cloud_storage` | `uri`, `managed`, `name`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at` |
| `chalk_unmanaged_cloud_storage` | `managed`, `name`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at` |
| `chalk_managed_container_registry` | `name`, `kind`, `managed`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at` |
| `chalk_unmanaged_container_registry` | `kind`, `managed`, `designator`, `team_id`, `applied_at`, `created_at`, `updated_at` |
| `chalk_cluster_*_cloud_storage_binding` (all five roles) | `created_at`, `updated_at` |
| `chalk_environment_*_cloud_storage_binding` (all four roles) | `created_at`, `updated_at` |
| `chalk_managed_cluster` | `name`, `kind`, `designator` |
| `chalk_managed_aws_vpc` | `name`, `designator` |
| `chalk_managed_gcp_vpc` | `name`, `designator` |
| `chalk_project` | `team_id` |
| `chalk_kubernetes_cluster` | `team_id` |
| `chalk_scaling_group` | `status`, `status_message`, `web_url` |
| `chalk_unmanaged_cluster_background_persistence` | `writers[*].name` |

Notes:

* On `chalk_unmanaged_cloud_storage` and `chalk_unmanaged_container_registry`, the
  `uri`/`name`/`kind` attributes that remain are the ones you configure as inputs;
  only the server-echoed copies were removed.
* `writers[*].name` on `chalk_unmanaged_cluster_background_persistence` was derived
  by the server from `bus_subscriber_type` and is still derived the same way — it is
  just no longer mirrored into Terraform state.

## What was kept

* **`id` on every resource.** It is the resource's identity: `terraform import` uses
  it, and it is how resources reference each other (bindings take `cluster_id`,
  `environment_id`, `cloud_storage_id`, and so on).
* **`chalk_service_token.client_id` and `client_secret`.** These are the entire
  output of the resource, the secret is only returned at creation and can never be
  re-fetched, and import uses `client_id`.
* **Every `Optional` + `Computed` attribute** (for example the
  `chalk_cluster_timescale` and `chalk_cluster_gateway` settings, or cloud storage
  `kind`). These are inputs with server-side defaults, not read-only attributes, and
  their behavior is unchanged.

## Migration steps

1. Search your configuration for references to the removed attributes, e.g.:

   ```sh
   grep -rnE 'chalk_[a-z_]+\.[^.]+\.(created_at|updated_at|applied_at|team_id|designator|managed|status|status_message|web_url)' .
   ```

2. Delete each reference (or replace it with data obtained outside Terraform).
   References to a removed `uri`/`name` on the *managed* storage/registry resources
   have no in-provider replacement — the values are visible in the Chalk dashboard.

3. Upgrade the provider and run `terraform plan`. A clean configuration plans with
   no changes; state is rewritten without the removed attributes on the next
   refresh.
