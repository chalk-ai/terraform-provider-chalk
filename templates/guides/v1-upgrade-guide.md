---
subcategory: ""
page_title: "Chalk Provider: Upgrade Guide for Version 1.0.0"
description: |-
  This guide covers the breaking changes introduced in v1.0.0 of the Chalk provider and what you need to do to upgrade.
---

# Upgrading to v1.0.0 of the Chalk provider

Version 1.0.0 removes two resources and one attribute that were deprecated in earlier 
releases. All three already have shipped, non-deprecated replacements, so there is no
functionality being dropped. Run `terraform plan` after upgrading; if your configuration
still references any of the items below, Terraform will fail to plan, and you'll need to
make the changes described here before you can plan or apply.

## Breaking change: `environment_buckets` removed

The `environment_buckets` attribute has been removed from `chalk_managed_environment`
and `chalk_unmanaged_environment` (it was marked deprecated starting in v0.9.x). It is
replaced by four environment-scoped cloud storage binding resources:

* `chalk_environment_dataset_cloud_storage_binding`
* `chalk_environment_plan_stages_cloud_storage_binding`
* `chalk_environment_source_bundle_cloud_storage_binding`
* `chalk_environment_model_registry_cloud_storage_binding`

Each binding points at a `chalk_managed_cloud_storage` or `chalk_unmanaged_cloud_storage`
resource rather than a bare bucket URI, so you'll first register the bucket itself, then
bind it to the environment.

Before:

```hcl
resource "chalk_unmanaged_environment" "example" {
  name            = "example"
  project_id      = chalk_project.example.id
  kube_cluster_id = chalk_kubernetes_cluster.example.id

  environment_buckets = {
    dataset_bucket        = "s3://my-chalk-datasets"
    plan_stages_bucket    = "s3://my-chalk-stages"
    source_bundle_bucket  = "s3://my-chalk-source"
    model_registry_bucket = "s3://my-chalk-models"
  }
}
```

After:

```hcl
resource "chalk_unmanaged_environment" "example" {
  name            = "example"
  project_id      = chalk_project.example.id
  kube_cluster_id = chalk_kubernetes_cluster.example.id
}

resource "chalk_unmanaged_cloud_storage" "dataset" {
  uri                 = "s3://my-chalk-datasets"
  cloud_credential_id = chalk_aws_cloud_credentials.example.id
}

resource "chalk_unmanaged_cloud_storage" "plan_stages" {
  uri                 = "s3://my-chalk-stages"
  cloud_credential_id = chalk_aws_cloud_credentials.example.id
}

resource "chalk_unmanaged_cloud_storage" "source_bundle" {
  uri                 = "s3://my-chalk-source"
  cloud_credential_id = chalk_aws_cloud_credentials.example.id
}

resource "chalk_unmanaged_cloud_storage" "model_registry" {
  uri                 = "s3://my-chalk-models"
  cloud_credential_id = chalk_aws_cloud_credentials.example.id
}

resource "chalk_environment_dataset_cloud_storage_binding" "dataset" {
  environment_id   = chalk_unmanaged_environment.example.id
  cloud_storage_id = chalk_unmanaged_cloud_storage.dataset.id
}

resource "chalk_environment_plan_stages_cloud_storage_binding" "plan_stages" {
  environment_id   = chalk_unmanaged_environment.example.id
  cloud_storage_id = chalk_unmanaged_cloud_storage.plan_stages.id
}

resource "chalk_environment_source_bundle_cloud_storage_binding" "source_bundle" {
  environment_id   = chalk_unmanaged_environment.example.id
  cloud_storage_id = chalk_unmanaged_cloud_storage.source_bundle.id
}

resource "chalk_environment_model_registry_cloud_storage_binding" "model_registry" {
  environment_id   = chalk_unmanaged_environment.example.id
  cloud_storage_id = chalk_unmanaged_cloud_storage.model_registry.id
}
```

## Breaking change: `chalk_environment` resource removed

The legacy `chalk_environment` resource has been removed. Use `chalk_managed_environment`
(if you set `managed = true`) or `chalk_unmanaged_environment` (otherwise) instead.

## Breaking change: `chalk_cluster_background_persistence` resource removed

The legacy `chalk_cluster_background_persistence` resource has been removed. Use
`chalk_unmanaged_cluster_background_persistence` instead. 
