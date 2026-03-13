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

func setupMockServerClusterGatewayBinding(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingClusterGateway().Return(&serverv1.CreateBindingClusterGatewayResponse{})
	server.OnGetBindingClusterGateway().Return(&serverv1.GetBindingClusterGatewayResponse{
		ClusterId:        "test-cluster-id",
		ClusterGatewayId: "test-gateway-id",
	})
	server.OnDeleteBindingClusterGateway().Return(&serverv1.DeleteBindingClusterGatewayResponse{})

	return server
}

// TestClusterGatewayBindingCreate verifies the basic create/read/delete lifecycle.
func TestClusterGatewayBindingCreate(t *testing.T) {
	t.Parallel()
	server := setupMockServerClusterGatewayBinding(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cluster_gateway_binding" "test" {
  cluster_id         = "test-cluster-id"
  cluster_gateway_id = "test-gateway-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_gateway_binding.test", "cluster_id", "test-cluster-id"),
					resource.TestCheckResourceAttr("chalk_cluster_gateway_binding.test", "cluster_gateway_id", "test-gateway-id"),
				),
			},
		},
	})
}

// TestClusterGatewayBindingReadNotFound verifies that when Get returns not_found,
// the resource is removed from state so Terraform can detect drift and recreate it.
func TestClusterGatewayBindingReadNotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingClusterGateway().Return(&serverv1.CreateBindingClusterGatewayResponse{})
	server.OnDeleteBindingClusterGateway().Return(&serverv1.DeleteBindingClusterGatewayResponse{})

	var getCallCount int
	server.OnGetBindingClusterGateway().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("binding not found"))
		}
		return &serverv1.GetBindingClusterGatewayResponse{
			ClusterId:        "test-cluster-id",
			ClusterGatewayId: "test-gateway-id",
		}, nil
	})

	config := providerConfig(server.URL) + `
resource "chalk_cluster_gateway_binding" "test" {
  cluster_id         = "test-cluster-id"
  cluster_gateway_id = "test-gateway-id"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Step 1: create the binding
			{Config: config},
			// Step 2: refresh state — Get returns not_found, resource is removed, plan shows diff
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
