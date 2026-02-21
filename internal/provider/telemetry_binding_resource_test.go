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

func setupMockServerTelemetryBinding(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingClusterTelemetryDeployment().Return(&serverv1.CreateBindingClusterTelemetryDeploymentResponse{})
	server.OnGetBindingClusterTelemetryDeployment().Return(&serverv1.GetBindingClusterTelemetryDeploymentResponse{
		ClusterId:             "test-cluster-id",
		TelemetryDeploymentId: "test-telemetry-id",
	})
	server.OnDeleteBindingClusterTelemetryDeployment().Return(&serverv1.DeleteBindingClusterTelemetryDeploymentResponse{})

	return server
}

// TestTelemetryBindingCreate verifies the basic create/read/delete lifecycle.
func TestTelemetryBindingCreate(t *testing.T) {
	server := setupMockServerTelemetryBinding(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_telemetry_binding" "test" {
  cluster_id              = "test-cluster-id"
  telemetry_deployment_id = "test-telemetry-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_telemetry_binding.test", "cluster_id", "test-cluster-id"),
					resource.TestCheckResourceAttr("chalk_telemetry_binding.test", "telemetry_deployment_id", "test-telemetry-id"),
				),
			},
		},
	})
}

// TestTelemetryBindingReadNotFound verifies that when Get returns not_found,
// the resource is removed from state so Terraform can detect drift and recreate it.
func TestTelemetryBindingReadNotFound(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	setupTestEnv(t, server.URL)

	server.OnCreateBindingClusterTelemetryDeployment().Return(&serverv1.CreateBindingClusterTelemetryDeploymentResponse{})
	server.OnDeleteBindingClusterTelemetryDeployment().Return(&serverv1.DeleteBindingClusterTelemetryDeploymentResponse{})

	var getCallCount int
	server.OnGetBindingClusterTelemetryDeployment().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("binding not found"))
		}
		return &serverv1.GetBindingClusterTelemetryDeploymentResponse{
			ClusterId:             "test-cluster-id",
			TelemetryDeploymentId: "test-telemetry-id",
		}, nil
	})

	config := `
resource "chalk_telemetry_binding" "test" {
  cluster_id              = "test-cluster-id"
  telemetry_deployment_id = "test-telemetry-id"
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
