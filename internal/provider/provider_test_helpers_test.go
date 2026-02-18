package provider

import (
	"os"
	"testing"

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
	os.Exit(m.Run())
}

func setupTestEnv(t *testing.T, serverURL string) {
	os.Setenv("CHALK_API_SERVER", serverURL)
	os.Setenv("CHALK_CLIENT_ID", "test-client-id")
	os.Setenv("CHALK_CLIENT_SECRET", "test-client-secret")
	t.Cleanup(func() {
		os.Unsetenv("CHALK_API_SERVER")
		os.Unsetenv("CHALK_CLIENT_ID")
		os.Unsetenv("CHALK_CLIENT_SECRET")
	})
}

// testProtoV6ProviderFactories configures the provider to use a mock server.
func testProtoV6ProviderFactories(mockServerURL string) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"chalk": providerserver.NewProtocol6WithError(func() provider.Provider {
			return &ChalkProvider{version: "test"}
		}()),
	}
}
