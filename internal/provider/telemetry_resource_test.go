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
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
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
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
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
				Config: providerConfig(server.URL) + `
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
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
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
				Config: providerConfig(server.URL) + `
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
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
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
				Config: providerConfig(server.URL) + `
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

// TestTelemetryResourceUpdateAggregatorFieldMask verifies that only "aggregator" is in the mask when aggregator changes.
func TestTelemetryResourceUpdateAggregatorFieldMask(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
  aggregator_spec = {
    image_version = "1.0.0"
  }
}
`,
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
  aggregator_spec = {
    image_version = "1.1.0"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateTelemetryDeployment")
						require.NotEmpty(t, captured, "Expected at least one UpdateTelemetryDeployment call")

						req := captured[len(captured)-1].(*serverv1.UpdateTelemetryDeploymentRequest)
						assert.NotNil(t, req.UpdateMask, "Expected UpdateMask to be set")
						assert.Equal(t, []string{"aggregator"}, req.UpdateMask.Paths,
							"Expected only 'aggregator' in field mask")
						require.NotNil(t, req.Spec.Aggregator)
						assert.Equal(t, "1.1.0", req.Spec.Aggregator.ImageVersion)

						return nil
					},
				),
			},
		},
	})
}

// TestTelemetryResourceNoOpUpdate verifies no RPC call is made when no fields change.
func TestTelemetryResourceNoOpUpdate(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)

	config := providerConfig(server.URL) + `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
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

// TestTelemetryResourceServerDefaults verifies that when the server returns defaults for unconfigured
// specs, the provider does not show drift on subsequent plans (the original bug).
func TestTelemetryResourceServerDefaults(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	const deploymentID = "test-telemetry-id"
	const clusterID = "test-cluster-id"

	// Server always returns all three specs with defaults, regardless of what was sent.
	serverDefaultSpec := &serverv1.TelemetryDeploymentSpec{
		ClickHouse: &serverv1.ClickHouseSpec{
			ClickHouseVersion: "23.8",
		},
		Otel: &serverv1.OtelCollectorSpec{
			OtelCollectorVersion: "0.88.0",
		},
		Aggregator: &serverv1.AggregatorSpec{
			ImageVersion: "1.0.0",
		},
	}

	server.OnCreateTelemetryDeployment().Return(&serverv1.CreateTelemetryDeploymentResponse{
		TelemetryDeploymentId: deploymentID,
	})
	server.OnGetTelemetryDeployment().Return(&serverv1.GetTelemetryDeploymentResponse{
		Deployment: &serverv1.TelemetryDeployment{
			Id:        deploymentID,
			ClusterId: clusterID,
			Spec:      serverDefaultSpec,
		},
	})
	server.OnDeleteTelemetryDeployment().Return(&serverv1.DeleteTelemetryDeploymentResponse{})

	// User only configures clickhouse; server fills in otel and aggregator as defaults.
	config := providerConfig(server.URL) + `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Step 1: apply should succeed (no "inconsistent result after apply" error).
			{Config: config},
			// Step 2: re-plan with same config should show no changes despite server returning defaults.
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestTelemetryResourceCreateError verifies proper error handling when Create RPC fails.
func TestTelemetryResourceCreateError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	server.OnCreateTelemetryDeployment().ReturnError(
		connect.NewError(connect.CodeInvalidArgument, errors.New("clickhouse-version-invalid")))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
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
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)

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
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  clickhouse_deployment_spec = {
    version = "23.8"
  }
}
`,
			},
			{
				Config: providerConfig(server.URL) + `
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

// TestTelemetryResourceCustomerVectorAggregator verifies both exporters round-trip and
// that a signal's presence, not its value, is what the server sees as "export this".
func TestTelemetryResourceCustomerVectorAggregator(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  customer_vector_aggregator = {
    datadog_export = {
      api_key_secret_reference = "arn:aws:secretsmanager:us-west-2:123456789012:secret:dd-abc123"
      api_host                 = "datadoghq.eu"
      logs                     = { enabled = true }
      traces                   = {}
      metrics                  = { enabled = false }
    }
    otlp_metrics_export = {
      url                                   = "https://otlp.example.com/v1/metrics"
      authorization_header_secret_reference = "arn:aws:secretsmanager:us-west-2:123456789012:secret:otlp-def456"
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_telemetry.test", "customer_vector_aggregator.datadog_export.api_host", "datadoghq.eu"),
					resource.TestCheckResourceAttr("chalk_telemetry.test", "customer_vector_aggregator.datadog_export.logs.enabled", "true"),
					resource.TestCheckResourceAttr("chalk_telemetry.test", "customer_vector_aggregator.otlp_metrics_export.url", "https://otlp.example.com/v1/metrics"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateTelemetryDeployment")
						require.Len(t, captured, 1, "Expected exactly one CreateTelemetryDeployment call")

						req := captured[0].(*serverv1.CreateTelemetryDeploymentRequest)
						agg := req.Spec.GetCustomerVectorAggregator()
						require.NotNil(t, agg)

						dd := agg.GetDatadogExport()
						require.NotNil(t, dd)
						assert.Equal(t, "arn:aws:secretsmanager:us-west-2:123456789012:secret:dd-abc123", dd.GetApiKeySecretArn())
						assert.Equal(t, "datadoghq.eu", dd.GetApiHost())

						require.NotNil(t, dd.GetLogs())
						assert.Equal(t, true, dd.GetLogs().GetEnabled())
						// An empty block must still send a message: the server reads presence as
						// enabled, and a nil traces field would silently drop trace export.
						require.NotNil(t, dd.GetTraces())
						assert.Nil(t, dd.GetTraces().Enabled)
						require.NotNil(t, dd.GetMetrics())
						assert.Equal(t, false, dd.GetMetrics().GetEnabled())

						otlp := agg.GetOtlpMetricsExport()
						require.NotNil(t, otlp)
						assert.Equal(t, "https://otlp.example.com/v1/metrics", otlp.GetUrl())
						assert.Equal(t, "arn:aws:secretsmanager:us-west-2:123456789012:secret:otlp-def456", otlp.GetAuthorizationHeaderSecretArn())

						return nil
					},
				),
			},
		},
	})
}

// TestTelemetryResourceCustomerVectorAggregatorFieldMask verifies the mask names the two
// exporters rather than their parent, so unmanaged siblings on that message survive.
func TestTelemetryResourceCustomerVectorAggregatorFieldMask(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  customer_vector_aggregator = {
    datadog_export = {
      api_key_secret_reference = "arn:aws:secretsmanager:us-west-2:123456789012:secret:dd-abc123"
      logs                     = { enabled = true }
    }
  }
}
`,
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
  customer_vector_aggregator = {
    datadog_export = {
      api_key_secret_reference = "arn:aws:secretsmanager:us-west-2:123456789012:secret:dd-abc123"
      logs                     = { enabled = false }
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateTelemetryDeployment")
						require.NotEmpty(t, captured, "Expected at least one UpdateTelemetryDeployment call")

						req := captured[len(captured)-1].(*serverv1.UpdateTelemetryDeploymentRequest)
						require.NotNil(t, req.UpdateMask)
						assert.Equal(t, []string{
							"customer_vector_aggregator.datadog_export",
							"customer_vector_aggregator.otlp_metrics_export",
						}, req.UpdateMask.Paths)

						return nil
					},
				),
			},
		},
	})
}

// TestTelemetryResourceCustomerVectorAggregatorOmitted verifies that a deployment carrying
// only fields this resource does not model reads back as unset instead of drifting.
func TestTelemetryResourceCustomerVectorAggregatorOmitted(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)

	server.OnGetTelemetryDeployment().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.GetTelemetryDeploymentResponse{
			Deployment: &serverv1.TelemetryDeployment{
				Id:        "test-telemetry-id",
				ClusterId: "test-cluster-id",
				Spec: &serverv1.TelemetryDeploymentSpec{
					CustomerVectorAggregator: &serverv1.CustomerVectorAggregatorConfig{
						Replicas: proto.Int32(3),
					},
				},
			},
		}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_telemetry" "test" {
  kube_cluster_id = "test-cluster-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("chalk_telemetry.test", "customer_vector_aggregator.datadog_export"),
				),
			},
		},
	})
}
