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

func setupMockServerEnvironmentBGPersistenceBinding(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingEnvironmentBackgroundPersistenceDeployment().Return(&serverv1.CreateBindingEnvironmentBackgroundPersistenceDeploymentResponse{})
	server.OnGetBindingEnvironmentBackgroundPersistenceDeployment().Return(&serverv1.GetBindingEnvironmentBackgroundPersistenceDeploymentResponse{
		EnvironmentId:                     "test-environment-id",
		BackgroundPersistenceDeploymentId: "test-bg-persist-id",
	})
	server.OnDeleteBindingEnvironmentBackgroundPersistenceDeployment().Return(&serverv1.DeleteBindingEnvironmentBackgroundPersistenceDeploymentResponse{})

	return server
}

// TestEnvironmentBackgroundPersistenceDeploymentBindingCreate verifies the basic create/read/delete lifecycle.
func TestEnvironmentBackgroundPersistenceDeploymentBindingCreate(t *testing.T) {
	server := setupMockServerEnvironmentBGPersistenceBinding(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_environment_background_persistence_deployment_binding" "test" {
  environment_id                       = "test-environment-id"
  background_persistence_deployment_id = "test-bg-persist-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_environment_background_persistence_deployment_binding.test", "environment_id", "test-environment-id"),
					resource.TestCheckResourceAttr("chalk_environment_background_persistence_deployment_binding.test", "background_persistence_deployment_id", "test-bg-persist-id"),
				),
			},
		},
	})
}

// TestEnvironmentBackgroundPersistenceDeploymentBindingReadNotFound verifies that when Get returns not_found,
// the resource is removed from state so Terraform can detect drift and recreate it.
func TestEnvironmentBackgroundPersistenceDeploymentBindingReadNotFound(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	setupTestEnv(t, server.URL)

	server.OnCreateBindingEnvironmentBackgroundPersistenceDeployment().Return(&serverv1.CreateBindingEnvironmentBackgroundPersistenceDeploymentResponse{})
	server.OnDeleteBindingEnvironmentBackgroundPersistenceDeployment().Return(&serverv1.DeleteBindingEnvironmentBackgroundPersistenceDeploymentResponse{})

	var getCallCount int
	server.OnGetBindingEnvironmentBackgroundPersistenceDeployment().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("binding not found"))
		}
		return &serverv1.GetBindingEnvironmentBackgroundPersistenceDeploymentResponse{
			EnvironmentId:                     "test-environment-id",
			BackgroundPersistenceDeploymentId: "test-bg-persist-id",
		}, nil
	})

	config := `
resource "chalk_environment_background_persistence_deployment_binding" "test" {
  environment_id                       = "test-environment-id"
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
