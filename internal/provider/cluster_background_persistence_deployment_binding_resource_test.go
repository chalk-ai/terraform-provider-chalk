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

func setupMockServerBGPersistenceBinding(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingClusterBackgroundPersistenceDeployment().Return(&serverv1.CreateBindingClusterBackgroundPersistenceDeploymentResponse{})
	server.OnGetBindingClusterBackgroundPersistenceDeployment().Return(&serverv1.GetBindingClusterBackgroundPersistenceDeploymentResponse{
		ClusterId:                         "test-cluster-id",
		BackgroundPersistenceDeploymentId: "test-bg-persist-id",
	})
	server.OnDeleteBindingClusterBackgroundPersistenceDeployment().Return(&serverv1.DeleteBindingClusterBackgroundPersistenceDeploymentResponse{})

	return server
}

// TestClusterBackgroundPersistenceDeploymentBindingCreate verifies the basic create/read/delete lifecycle.
func TestClusterBackgroundPersistenceDeploymentBindingCreate(t *testing.T) {
	server := setupMockServerBGPersistenceBinding(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_cluster_background_persistence_deployment_binding" "test" {
  cluster_id                           = "test-cluster-id"
  background_persistence_deployment_id = "test-bg-persist-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_background_persistence_deployment_binding.test", "cluster_id", "test-cluster-id"),
					resource.TestCheckResourceAttr("chalk_cluster_background_persistence_deployment_binding.test", "background_persistence_deployment_id", "test-bg-persist-id"),
				),
			},
		},
	})
}

// TestClusterBackgroundPersistenceDeploymentBindingReadNotFound verifies that when Get returns not_found,
// the resource is removed from state so Terraform can detect drift and recreate it.
func TestClusterBackgroundPersistenceDeploymentBindingReadNotFound(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	setupTestEnv(t, server.URL)

	server.OnCreateBindingClusterBackgroundPersistenceDeployment().Return(&serverv1.CreateBindingClusterBackgroundPersistenceDeploymentResponse{})
	server.OnDeleteBindingClusterBackgroundPersistenceDeployment().Return(&serverv1.DeleteBindingClusterBackgroundPersistenceDeploymentResponse{})

	var getCallCount int
	server.OnGetBindingClusterBackgroundPersistenceDeployment().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("binding not found"))
		}
		return &serverv1.GetBindingClusterBackgroundPersistenceDeploymentResponse{
			ClusterId:                         "test-cluster-id",
			BackgroundPersistenceDeploymentId: "test-bg-persist-id",
		}, nil
	})

	config := `
resource "chalk_cluster_background_persistence_deployment_binding" "test" {
  cluster_id                           = "test-cluster-id"
  background_persistence_deployment_id = "test-bg-persist-id"
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
