# Terraform Provider for Chalk

This repository contains a Terraform provider for [Chalk](https://chalk.ai), enabling infrastructure-as-code management of Chalk resources.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21

## Building The Provider

1. Clone the repository
2. Enter the repository directory
3. Build the provider using the Go `install` command:

```shell
go install
```

## Using the Provider

### Provider Configuration

Configure the provider in your Terraform configuration:

```hcl
terraform {
  required_providers {
    chalk = {
      source = "chalk-ai/chalk"
      version = "~> 0.1"
    }
  }
}

provider "chalk" {
  api_token = var.chalk_api_token # or use CHALK_API_TOKEN environment variable
  api_url   = "https://api.chalk.ai" # optional, defaults to https://api.chalk.ai
}
```

### Authentication

The provider supports authentication via:
- `api_token` provider configuration
- `CHALK_API_TOKEN` environment variable

### Example Usage

Query environment information:

```hcl
data "chalk_environment" "prod" {
  name = "production"
}

output "environment_id" {
  value = data.chalk_environment.prod.id
}
```

## Testing

### Writing Tests

Tests are organized into two categories:

1. **Unit Tests**: Located alongside the source files (e.g., `provider_test.go`)
   ```go
   func TestAccEnvironmentDataSource(t *testing.T) {
       resource.Test(t, resource.TestCase{
           PreCheck:                 func() { testAccPreCheck(t) },
           ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
           Steps: []resource.TestStep{
               {
                   Config: testAccEnvironmentDataSourceConfig("production"),
                   Check: resource.ComposeAggregateTestCheckFunc(
                       resource.TestCheckResourceAttr("data.chalk_environment.test", "name", "production"),
                       resource.TestCheckResourceAttrSet("data.chalk_environment.test", "id"),
                   ),
               },
           },
       })
   }
   ```

2. **Acceptance Tests**: Run against a real Chalk API
   ```shell
   # Run acceptance tests
   make testacc
   
   # Run specific test
   TF_ACC=1 go test ./internal/provider -v -run TestAccEnvironmentDataSource
   ```

### Test Environment Setup

Before running acceptance tests:

```shell
export CHALK_API_TOKEN="your-api-token"
export TF_ACC=1
```

## Local Development

### Testing the Provider Locally

1. Build and install the provider locally:
   ```shell
   make install
   ```

2. Create a test Terraform configuration:
   ```hcl
   # main.tf
   terraform {
     required_providers {
       chalk = {
         source = "registry.terraform.io/chalk-ai/chalk"
         version = "0.1.0"
       }
     }
   }

   provider "chalk" {
     api_token = "your-test-token"
   }

   data "chalk_environment" "test" {
     name = "production"
   }

   output "env_info" {
     value = data.chalk_environment.test
   }
   ```

3. Run Terraform commands:
   ```shell
   terraform init
   terraform plan
   ```

### Development Override

For active development, use a Terraform CLI configuration file to override the provider:

1. Create `~/.terraformrc`:
   ```hcl
   provider_installation {
     dev_overrides {
       "chalk-ai/chalk" = "/Users/yourusername/go/bin"
     }
     direct {}
   }
   ```

2. Build the provider:
   ```shell
   go build -o /Users/yourusername/go/bin/terraform-provider-chalk
   ```

3. Now Terraform will use your local build without requiring `terraform init`.

## Available Resources and Data Sources

### Data Sources

- `chalk_environment` - Query information about a Chalk environment

## Contributing

### Development Workflow

1. Make your changes
2. Run tests: `make test`
3. Run linter: `make lint`
4. Format code: `make fmt`
5. Build provider: `make build`

### Debugging

To run the provider with debug output:

```shell
# Build with debug flag
go build -gcflags="all=-N -l" -o terraform-provider-chalk

# Run Terraform with debug logging
TF_LOG=DEBUG terraform plan
```

To attach a debugger:

```shell
# Start provider in debug mode
./terraform-provider-chalk -debug

# Provider will output something like:
# Provider started, to attach Terraform set the TF_REATTACH_PROVIDERS env var:
# TF_REATTACH_PROVIDERS='{"chalk-ai/chalk":{"Protocol":"grpc","Pid":12345,"Test":true,"Addr":{"Network":"unix","String":"/tmp/plugin123456"}}}'

# In another terminal, use the provided environment variable
export TF_REATTACH_PROVIDERS='...'
terraform plan
```

## Makefile Commands

- `make build` - Build the provider
- `make install` - Build and install the provider locally
- `make test` - Run unit tests
- `make testacc` - Run acceptance tests
- `make fmt` - Format Go code and Terraform examples
- `make lint` - Run linters

## Documentation

Provider documentation is generated from the schema descriptions. To view the documentation for a data source or resource, refer to the schema definitions in the source code.

## License

This provider is distributed under the [MIT License](LICENSE).