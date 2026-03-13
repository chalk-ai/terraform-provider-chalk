package provider

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"google.golang.org/protobuf/proto"
)

func setupMockServerEnvironmentOfflineStoreConnectionBinding(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingEnvironmentOfflineStoreConnection().Return(&serverv1.CreateBindingEnvironmentOfflineStoreConnectionResponse{})
	server.OnGetBindingEnvironmentOfflineStoreConnection().Return(&serverv1.GetBindingEnvironmentOfflineStoreConnectionResponse{
		EnvironmentId:            "test-environment-id",
		OfflineStoreConnectionId: "test-connection-id",
	})
	server.OnDeleteBindingEnvironmentOfflineStoreConnection().Return(&serverv1.DeleteBindingEnvironmentOfflineStoreConnectionResponse{})

	return server
}

// TestEnvironmentOfflineStoreConnectionBindingCreate verifies the basic create/read/delete lifecycle.
func TestEnvironmentOfflineStoreConnectionBindingCreate(t *testing.T) {
	t.Parallel()
	server := setupMockServerEnvironmentOfflineStoreConnectionBinding(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_environment_offline_store_connection_binding" "test" {
  environment_id              = "test-environment-id"
  offline_store_connection_id = "test-connection-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_environment_offline_store_connection_binding.test", "environment_id", "test-environment-id"),
					resource.TestCheckResourceAttr("chalk_environment_offline_store_connection_binding.test", "offline_store_connection_id", "test-connection-id"),
				),
			},
		},
	})
}

// TestEnvironmentOfflineStoreConnectionBindingReadNotFound verifies that a CodeNotFound on Get
// removes the resource from state so Terraform can detect drift and recreate it.
func TestEnvironmentOfflineStoreConnectionBindingReadNotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingEnvironmentOfflineStoreConnection().Return(&serverv1.CreateBindingEnvironmentOfflineStoreConnectionResponse{})
	server.OnDeleteBindingEnvironmentOfflineStoreConnection().Return(&serverv1.DeleteBindingEnvironmentOfflineStoreConnectionResponse{})

	var getCallCount int
	server.OnGetBindingEnvironmentOfflineStoreConnection().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("binding not found"))
		}
		return &serverv1.GetBindingEnvironmentOfflineStoreConnectionResponse{
			EnvironmentId:            "test-environment-id",
			OfflineStoreConnectionId: "test-connection-id",
		}, nil
	})

	config := providerConfig(server.URL) + `
resource "chalk_environment_offline_store_connection_binding" "test" {
  environment_id              = "test-environment-id"
  offline_store_connection_id = "test-connection-id"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: config},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
