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

func setupMockServerPrivateGatewayBinding(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingPrivateGateway().Return(&serverv1.CreateBindingPrivateGatewayResponse{})
	server.OnGetBindingPrivateGateway().Return(&serverv1.GetBindingPrivateGatewayResponse{
		ClusterId:        "test-cluster-id",
		PrivateGatewayId: "test-private-gateway-id",
	})
	server.OnDeleteBindingPrivateGateway().Return(&serverv1.DeleteBindingPrivateGatewayResponse{})

	return server
}

// TestPrivateGatewayBindingCreate verifies the basic create/read/delete lifecycle.
func TestPrivateGatewayBindingCreate(t *testing.T) {
	server := setupMockServerPrivateGatewayBinding(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_private_gateway_binding" "test" {
  cluster_id         = "test-cluster-id"
  private_gateway_id = "test-private-gateway-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_private_gateway_binding.test", "cluster_id", "test-cluster-id"),
					resource.TestCheckResourceAttr("chalk_private_gateway_binding.test", "private_gateway_id", "test-private-gateway-id"),
				),
			},
		},
	})
}

// TestPrivateGatewayBindingReadNotFound verifies that when Get returns not_found,
// the resource is removed from state so Terraform can detect drift and recreate it.
func TestPrivateGatewayBindingReadNotFound(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	setupTestEnv(t, server.URL)

	server.OnCreateBindingPrivateGateway().Return(&serverv1.CreateBindingPrivateGatewayResponse{})
	server.OnDeleteBindingPrivateGateway().Return(&serverv1.DeleteBindingPrivateGatewayResponse{})

	var getCallCount int
	server.OnGetBindingPrivateGateway().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("binding not found"))
		}
		return &serverv1.GetBindingPrivateGatewayResponse{
			ClusterId:        "test-cluster-id",
			PrivateGatewayId: "test-private-gateway-id",
		}, nil
	})

	config := `
resource "chalk_private_gateway_binding" "test" {
  cluster_id         = "test-cluster-id"
  private_gateway_id = "test-private-gateway-id"
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