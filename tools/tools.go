//go:build generate

package tools

import (
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
	_ "gotest.tools/gotestsum"
)

// Format Terraform code in examples.
//go:generate terraform fmt -recursive ../examples/

// Generate permissions_gen.go (must run before tfplugindocs builds the provider).
//go:generate go run ../tools/genpermissions --provider-dir ..

// Generate documentation.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-dir .. --provider-name chalk
