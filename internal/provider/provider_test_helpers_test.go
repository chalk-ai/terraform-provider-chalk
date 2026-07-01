package provider

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// TestMain sets up the test environment for all tests in this package.
// We set TF_ACC=1 to enable the Terraform testing framework (resource.Test).
//
// Note: These are NOT traditional acceptance tests that require real infrastructure.
// They are integration tests that:
//   - Use mock servers (chalk-go/testserver) instead of real APIs
//   - Test the full Terraform lifecycle (plan/apply/refresh/destroy)
//   - Verify RPC calls, field masks, and state management
//   - Are fast, safe, and require no credentials
//   - Should run on every `go test` invocation
//
// We use resource.Test() because it catches real integration bugs (e.g., computed
// field handling, state management) that wouldn't be found in simple unit tests.
func TestMain(m *testing.M) {
	os.Setenv("TF_ACC", "1")
	// Shorten managed-resource poll intervals so tests that exercise the
	// apply/delete polling loops run quickly. The production timeouts are left
	// untouched. Set here (not per-test) to avoid races between parallel tests
	// mutating these shared package vars.
	vpcPollInterval = time.Millisecond
	clusterPollInterval = time.Millisecond
	os.Exit(m.Run())
}

func providerConfig(serverURL string) string {
	return fmt.Sprintf(`
provider "chalk" {
  api_server    = %q
  client_id     = "test-client-id"
  client_secret = "test-client-secret"
}
`, serverURL)
}

// testProtoV6ProviderFactories configures the provider to use a mock server.
func testProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"chalk": providerserver.NewProtocol6WithError(func() provider.Provider {
			return &ChalkProvider{version: "test"}
		}()),
	}
}
