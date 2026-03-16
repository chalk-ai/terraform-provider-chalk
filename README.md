# Terraform Provider for Chalk

This repository contains a Terraform provider for [Chalk](https://chalk.ai), enabling infrastructure-as-code management of Chalk resources.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.26 (for development only)

## Usage

```hcl
terraform {
  required_providers {
    chalk = {
      source  = "chalk-ai/chalk"
      version = "~> 0.9"
    }
  }
}

provider "chalk" {
  api_server    = "https://api.chalk.ai"
  client_id     = var.chalk_client_id
  client_secret = var.chalk_client_secret
}
```

See [`terraform-provider-e2e`](https://github.com/chalk-ai/chalk-terraform/tree/main/terraform-provider-e2e) for a full working example.

## Local Development

### Build and install locally

```shell
make install
```

This builds the binary and installs it under `~/.terraform.d/plugins/` so Terraform picks it up automatically.

### Development override

For a faster iteration loop, use `dev_overrides` in `~/.terraformrc` to bypass `terraform init`:

```hcl
provider_installation {
  dev_overrides {
    "chalk-ai/chalk" = "/path/to/terraform-provider-chalk"
  }
  direct {}
}
```

Then just `go build .` and run `terraform plan` directly.

### Debugging

See the [Terraform plugin debugging guide](https://developer.hashicorp.com/terraform/plugin/debugging) for full instructions. The provider supports the reattach method — build with optimizations disabled and start with `-debug`:

```shell
go build -gcflags="all=-N -l" -o terraform-provider-chalk .
./terraform-provider-chalk -debug
```

The provider prints a `TF_REATTACH_PROVIDERS` value to stdout. Export it in a second terminal, then run Terraform commands normally:

```shell
export TF_REATTACH_PROVIDERS='...'
terraform plan
```

For verbose Terraform logging:

```shell
TF_LOG=DEBUG terraform plan
```

## Testing

```shell
make test    # unit tests (uses testserver, no real API needed)
```

## Other commands

```shell
make fmt            # format Go and Terraform example files
make docs           # regenerate provider documentation
make setup-hooks    # install pre-commit hooks via prek
make release        # tag and publish a new release
```

## Documentation

Provider documentation is generated from schema descriptions. To regenerate after making changes:

```shell
make docs
```

## CI

PRs are validated via Buildkite. The pipeline runs tests, linting, and formatting checks,
then triggers an E2E smoke test against a live Chalk environment using the provider binary
built from the PR commit.

Pipeline configs live in `.buildkite/`. Changes merged to `main` are automatically
reconciled by the `sync-buildkite` GitHub Actions workflow — no manual steps required.

### One-time setup (per repo)

Set the following in GitHub repo settings (done once, never again):

- **Secret** `BUILDKITE_API_TOKEN` — Buildkite API token with pipeline write access
- **Variable** `BUILDKITE_CLUSTER_ID` — `7b9e4371-390a-4f64-88b8-b281a34c0843`

`GITHUB_TOKEN` is provided automatically by GitHub Actions.

### Adopting in other repos

1. Add `.buildkite/*.yml` files with `on:` trigger blocks (`pull_request`, `push`, or `tag`)
2. Add `.github/workflows/sync-buildkite.yml` using `chalk-ai/buildkite-sync-action@v1`
3. Set `BUILDKITE_API_TOKEN` and `BUILDKITE_CLUSTER_ID` in GitHub repo settings