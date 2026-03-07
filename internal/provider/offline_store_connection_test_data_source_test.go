package provider

import (
	"errors"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const testOfflineStoreConnectionTestConfig = `
data "chalk_offline_store_connection_test" "test" {
  environment_id              = "test-env-id"
  offline_store_connection_id = "test-conn-id"
}
`

// TestOfflineStoreConnectionTestDataSourceSuccess verifies that the data source
// correctly reflects a passing connection test.
func TestOfflineStoreConnectionTestDataSourceSuccess(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	setupTestEnv(t, server.URL)

	server.OnTestOfflineStoreConnection().Return(&serverv1.TestOfflineStoreConnectionResponse{
		Success: true,
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: testOfflineStoreConnectionTestConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection_test.test", "success", "true"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection_test.test", "error_message", ""),
				),
			},
		},
	})
}

// TestOfflineStoreConnectionTestDataSourceFailure verifies that a failed connection test
// sets success=false and populates error_message from the server response.
func TestOfflineStoreConnectionTestDataSourceFailure(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	setupTestEnv(t, server.URL)

	server.OnTestOfflineStoreConnection().Return(&serverv1.TestOfflineStoreConnectionResponse{
		Success: false,
		Error:   "invalid credentials",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: testOfflineStoreConnectionTestConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection_test.test", "success", "false"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection_test.test", "error_message", "invalid credentials"),
				),
			},
		},
	})
}

// TestOfflineStoreConnectionTestDataSourceAPIError verifies that an API-level error
// (e.g. network failure, auth error) surfaces as a diagnostic error rather than success=false.
func TestOfflineStoreConnectionTestDataSourceAPIError(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	setupTestEnv(t, server.URL)

	server.OnTestOfflineStoreConnection().ReturnError(
		connect.NewError(connect.CodeInternal, errors.New("connection refused")),
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config:      testOfflineStoreConnectionTestConfig,
				ExpectError: regexp.MustCompile(`Could not test offline store connection`),
			},
		},
	})
}
