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

func setupMockServerEnvironmentGatewayBinding(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingEnvironmentGateway().Return(&serverv1.CreateBindingEnvironmentGatewayResponse{})
	server.OnGetBindingEnvironmentGateway().Return(&serverv1.GetBindingEnvironmentGatewayResponse{
		EnvironmentId:    "test-environment-id",
		ClusterGatewayId: "test-gateway-id",
	})
	server.OnDeleteBindingEnvironmentGateway().Return(&serverv1.DeleteBindingEnvironmentGatewayResponse{})

	return server
}

// TestEnvironmentGatewayBindingCreate verifies the basic create/read/delete lifecycle.
func TestEnvironmentGatewayBindingCreate(t *testing.T) {
	server := setupMockServerEnvironmentGatewayBinding(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_environment_gateway_binding" "test" {
  environment_id     = "test-environment-id"
  cluster_gateway_id = "test-gateway-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_environment_gateway_binding.test", "environment_id", "test-environment-id"),
					resource.TestCheckResourceAttr("chalk_environment_gateway_binding.test", "cluster_gateway_id", "test-gateway-id"),
				),
			},
		},
	})
}

// TestEnvironmentGatewayBindingReadNotFound verifies that when Get returns not_found,
// the resource is removed from state so Terraform can detect drift and recreate it.
func TestEnvironmentGatewayBindingReadNotFound(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	setupTestEnv(t, server.URL)

	server.OnCreateBindingEnvironmentGateway().Return(&serverv1.CreateBindingEnvironmentGatewayResponse{})
	server.OnDeleteBindingEnvironmentGateway().Return(&serverv1.DeleteBindingEnvironmentGatewayResponse{})

	var getCallCount int
	server.OnGetBindingEnvironmentGateway().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("binding not found"))
		}
		return &serverv1.GetBindingEnvironmentGatewayResponse{
			EnvironmentId:    "test-environment-id",
			ClusterGatewayId: "test-gateway-id",
		}, nil
	})

	config := `
resource "chalk_environment_gateway_binding" "test" {
  environment_id     = "test-environment-id"
  cluster_gateway_id = "test-gateway-id"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{Config: config},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
