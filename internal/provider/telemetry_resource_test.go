package provider

import (
	"errors"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// setupMockBuilderServerTelemetry creates a mock server for telemetry resource tests.
// It tracks current spec state across Create/Update/Get operations.
func setupMockBuilderServerTelemetry(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	var currentSpec *serverv1.TelemetryDeploymentSpec
	const deploymentID = "test-telemetry-id"
	const clusterID = "test-cluster-id"

	server.OnCreateTelemetryDeployment().WithBehavior(func(req proto.Message) (proto.Message, error) {
		createReq := req.(*serverv1.CreateTelemetryDeploymentRequest)
		currentSpec = createReq.Spec
		return &serverv1.CreateTelemetryDeploymentResponse{
			TelemetryDeploymentId: deploymentID,
		}, nil
	})

	server.OnUpdateTelemetryDeployment().WithBehavior(func(req proto.Message) (proto.Message, error) {
		updateReq := req.(*serverv1.UpdateTelemetryDeploymentRequest)
		currentSpec = updateReq.Spec
		return &serverv1.UpdateTelemetryDeploymentResponse{
			Deployment: &serverv1.TelemetryDeployment{
				Id:        deploymentID,
				ClusterId: clusterID,
				Spec:      currentSpec,
			},
		}, nil
	})

	server.OnGetTelemetryDeployment().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if currentSpec == nil {
			return &serverv1.GetTelemetryDeploymentResponse{}, nil
		}
		return &serverv1.GetTelemetryDeploymentResponse{
			Deployment: &serverv1.TelemetryDeployment{
				Id:        deploymentID,
				ClusterId: clusterID,
				Spec:      currentSpec,
			},
		}, nil
	})

	server.OnDeleteTelemetryDeployment().Return(&serverv1.DeleteTelemetryDeploymentResponse{})

	return server
}

// TestTelemetryResourceCreate verifies that CreateTelemetryDeployment is called with correct spec.
func TestTelemetryResourceCreate(t *testing.T) {
	server := setupMockBuilderServerTelemetry(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
  otel_collector_spec = {
    version = "0.88.0"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_telemetry.test", "id", "test-telemetry-id"),
					resource.TestCheckResourceAttr("chalk_telemetry.test", "kube_cluster_id", "test-cluster-id"),
					resource.TestCheckResourceAttr("chalk_telemetry.test", "clickhouse_deployment_spec.version", "23.8"),
					resource.TestCheckResourceAttr("chalk_telemetry.test", "otel_collector_spec.version", "0.88.0"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateTelemetryDeployment")
						require.Len(t, captured, 1, "Expected exactly one CreateTelemetryDeployment call")

						req := captured[0].(*serverv1.CreateTelemetryDeploymentRequest)
						assert.Equal(t, "test-cluster-id", req.ClusterId)
						require.NotNil(t, req.Spec)
						require.NotNil(t, req.Spec.ClickHouse)
						assert.Equal(t, "23.8", req.Spec.ClickHouse.ClickHouseVersion)
						require.NotNil(t, req.Spec.Otel)
						assert.Equal(t, "0.88.0", req.Spec.Otel.OtelCollectorVersion)

						return nil
					},
				),
			},
		},
	})
}

// TestTelemetryResourceUpdate verifies that UpdateTelemetryDeployment is called for updates.
func TestTelemetryResourceUpdate(t *testing.T) {
	server := setupMockBuilderServerTelemetry(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_telemetry.test", "clickhouse_deployment_spec.version", "23.8"),
				),
			},
			{
				Config: `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "24.1"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_telemetry.test", "clickhouse_deployment_spec.version", "24.1"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateTelemetryDeployment")
						require.NotEmpty(t, captured, "Expected at least one UpdateTelemetryDeployment call")
						return nil
					},
				),
			},
		},
	})
}

// TestTelemetryResourceUpdateFieldMask verifies that the field mask contains only the changed top-level field.
func TestTelemetryResourceUpdateFieldMask(t *testing.T) {
	server := setupMockBuilderServerTelemetry(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
  otel_collector_spec = {
    version = "0.88.0"
  }
}
`,
			},
			{
				Config: `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "24.1"
  }
  otel_collector_spec = {
    version = "0.88.0"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateTelemetryDeployment")
						require.NotEmpty(t, captured, "Expected at least one UpdateTelemetryDeployment call")

						req := captured[len(captured)-1].(*serverv1.UpdateTelemetryDeploymentRequest)
						assert.NotNil(t, req.UpdateMask, "Expected UpdateMask to be set")
						assert.Equal(t, []string{"click_house"}, req.UpdateMask.Paths,
							"Expected only 'click_house' in field mask")
						require.NotNil(t, req.Spec.ClickHouse)
						assert.Equal(t, "24.1", req.Spec.ClickHouse.ClickHouseVersion)

						return nil
					},
				),
			},
		},
	})
}

// TestTelemetryResourceUpdateOtelFieldMask verifies that only "otel" is in the mask when otel changes.
func TestTelemetryResourceUpdateOtelFieldMask(t *testing.T) {
	server := setupMockBuilderServerTelemetry(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
  otel_collector_spec = {
    version = "0.88.0"
  }
}
`,
			},
			{
				Config: `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
  otel_collector_spec = {
    version = "0.89.0"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateTelemetryDeployment")
						require.NotEmpty(t, captured, "Expected at least one UpdateTelemetryDeployment call")

						req := captured[len(captured)-1].(*serverv1.UpdateTelemetryDeploymentRequest)
						assert.NotNil(t, req.UpdateMask, "Expected UpdateMask to be set")
						assert.Equal(t, []string{"otel"}, req.UpdateMask.Paths,
							"Expected only 'otel' in field mask")
						require.NotNil(t, req.Spec.Otel)
						assert.Equal(t, "0.89.0", req.Spec.Otel.OtelCollectorVersion)

						return nil
					},
				),
			},
		},
	})
}

// TestTelemetryResourceNoOpUpdate verifies no RPC call is made when no fields change.
func TestTelemetryResourceNoOpUpdate(t *testing.T) {
	server := setupMockBuilderServerTelemetry(t)
	setupTestEnv(t, server.URL)

	config := `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{Config: config},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						createCalls := server.GetCapturedRequests("CreateTelemetryDeployment")
						updateCalls := server.GetCapturedRequests("UpdateTelemetryDeployment")

						assert.Len(t, createCalls, 1, "Expected exactly one CreateTelemetryDeployment call")
						assert.Empty(t, updateCalls, "Expected no UpdateTelemetryDeployment calls for no-op update")

						return nil
					},
				),
			},
		},
	})
}

// TestTelemetryResourceCreateError verifies proper error handling when Create RPC fails.
func TestTelemetryResourceCreateError(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	server.OnCreateTelemetryDeployment().ReturnError(
		connect.NewError(connect.CodeInvalidArgument, errors.New("clickhouse-version-invalid")))
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "invalid"
  }
}
`,
				ExpectError: regexp.MustCompile("clickhouse-version-invalid"),
			},
		},
	})
}

// TestTelemetryResourceUpdateError verifies proper error handling when Update RPC fails.
func TestTelemetryResourceUpdateError(t *testing.T) {
	server := setupMockBuilderServerTelemetry(t)
	setupTestEnv(t, server.URL)

	server.Reset()
	server.OnCreateTelemetryDeployment().Return(&serverv1.CreateTelemetryDeploymentResponse{
		TelemetryDeploymentId: "test-telemetry-id",
	})
	server.OnGetTelemetryDeployment().Return(&serverv1.GetTelemetryDeploymentResponse{
		Deployment: &serverv1.TelemetryDeployment{
			Id:        "test-telemetry-id",
			ClusterId: "test-cluster-id",
			Spec: &serverv1.TelemetryDeploymentSpec{
				ClickHouse: &serverv1.ClickHouseSpec{
					ClickHouseVersion: "23.8",
				},
			},
		},
	})
	server.OnUpdateTelemetryDeployment().ReturnError(
		connect.NewError(connect.CodeResourceExhausted, errors.New("quota exceeded")))
	server.OnDeleteTelemetryDeployment().Return(&serverv1.DeleteTelemetryDeploymentResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
}
`,
			},
			{
				Config: `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "24.1"
  }
}
`,
				ExpectError: regexp.MustCompile("quota exceeded"),
			},
		},
	})
}
