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

func setupMockServerClusterContainerRegistryBinding(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingClusterContainerRegistry().Return(&serverv1.CreateBindingClusterContainerRegistryResponse{})
	server.OnGetBindingClusterContainerRegistry().Return(&serverv1.GetBindingClusterContainerRegistryResponse{
		ClusterId:           "test-cluster-id",
		ContainerRegistryId: new("test-registry-id"),
	})
	server.OnDeleteBindingClusterContainerRegistry().Return(&serverv1.DeleteBindingClusterContainerRegistryResponse{})

	return server
}

// TestClusterContainerRegistryBindingCreate verifies the basic create/read/delete lifecycle.
func TestClusterContainerRegistryBindingCreate(t *testing.T) {
	t.Parallel()
	server := setupMockServerClusterContainerRegistryBinding(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cluster_container_registry_binding" "test" {
  cluster_id            = "test-cluster-id"
  container_registry_id = "test-registry-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_container_registry_binding.test", "cluster_id", "test-cluster-id"),
					resource.TestCheckResourceAttr("chalk_cluster_container_registry_binding.test", "container_registry_id", "test-registry-id"),
				),
			},
		},
	})
}

// TestClusterContainerRegistryBindingReadNotFound verifies that when Get returns not_found,
// the resource is removed from state so Terraform can detect drift and recreate it.
func TestClusterContainerRegistryBindingReadNotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingClusterContainerRegistry().Return(&serverv1.CreateBindingClusterContainerRegistryResponse{})
	server.OnDeleteBindingClusterContainerRegistry().Return(&serverv1.DeleteBindingClusterContainerRegistryResponse{})

	var getCallCount int
	server.OnGetBindingClusterContainerRegistry().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("binding not found"))
		}
		return &serverv1.GetBindingClusterContainerRegistryResponse{
			ClusterId:           "test-cluster-id",
			ContainerRegistryId: new("test-registry-id"),
		}, nil
	})

	config := providerConfig(server.URL) + `
resource "chalk_cluster_container_registry_binding" "test" {
  cluster_id            = "test-cluster-id"
  container_registry_id = "test-registry-id"
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
