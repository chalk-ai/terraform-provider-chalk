package provider

import (
	"errors"
	"os"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// TestMain sets up the test environment for all tests in this package.
// We set TF_ACC=1 to enable the Terraform testing framework (resource.Test).
//
// Note: These are NOT traditional acceptance tests that require real infrastructure.
// They are integration tests that:
//   - Use mock servers (chalk-go/testserver) instead of real APIs
//   - Test the full Terraform lifecycle (plan/apply/refresh/destroy)
//   - Verify RPC calls, field masks, and state management
//   - Are fast, safe, and require no credentials
//   - Should run on every `go test` invocation
//
// We use resource.Test() because it catches real integration bugs (e.g., computed
// field handling, state management) that wouldn't be found in simple unit tests.
func TestMain(m *testing.M) {
	os.Setenv("TF_ACC", "1")
	os.Exit(m.Run())
}

func setupTestEnv(t *testing.T, serverURL string) {
	os.Setenv("CHALK_API_SERVER", serverURL)
	os.Setenv("CHALK_CLIENT_ID", "test-client-id")
	os.Setenv("CHALK_CLIENT_SECRET", "test-client-secret")
	t.Cleanup(func() {
		os.Unsetenv("CHALK_API_SERVER")
		os.Unsetenv("CHALK_CLIENT_ID")
		os.Unsetenv("CHALK_CLIENT_SECRET")
	})
}

// testProtoV6ProviderFactories configures the provider to use a mock server.
func testProtoV6ProviderFactories(mockServerURL string) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"chalk": providerserver.NewProtocol6WithError(func() provider.Provider {
			return &ChalkProvider{version: "test"}
		}()),
	}
}

// setupMockBuilderServer creates and configures a mock server for testing.
// It creates a stateful mock that tracks specs across Create/Update/Get operations.
func setupMockBuilderServer(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	// Track current specs in a closure variable
	var currentSpecs *serverv1.ClusterTimescaleSpecs

	// Configure Create to save specs and return them
	server.OnCreateClusterTimescaleDB().WithBehavior(func(req proto.Message) (proto.Message, error) {
		createReq := req.(*serverv1.CreateClusterTimescaleDBRequest)
		currentSpecs = createReq.Specs
		return &serverv1.CreateClusterTimescaleDBResponse{
			ClusterTimescaleId: "test-cluster-id",
			Specs:              currentSpecs,
		}, nil
	})

	// Configure Update to save specs and return them
	server.OnUpdateClusterTimescaleDB().WithBehavior(func(req proto.Message) (proto.Message, error) {
		updateReq := req.(*serverv1.UpdateClusterTimescaleDBRequest)
		currentSpecs = updateReq.Specs
		return &serverv1.UpdateClusterTimescaleDBResponse{
			ClusterTimescaleId: "test-cluster-id",
			Specs:              currentSpecs,
		}, nil
	})

	// Configure Get to return current specs
	server.OnGetClusterTimescaleDB().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if currentSpecs == nil {
			// Return default empty response if no create/update has happened yet
			return &serverv1.GetClusterTimescaleDBResponse{
				Id:    "",
				Specs: nil,
			}, nil
		}
		return &serverv1.GetClusterTimescaleDBResponse{
			Id:    "test-cluster-id",
			Specs: currentSpecs,
		}, nil
	})

	// Configure Delete
	server.OnDeleteClusterTimescaleDB().Return(&serverv1.DeleteClusterTimescaleDBResponse{})

	return server
}

// mockClusterTimescaleSpecs creates a ClusterTimescaleSpecs with default values.
func mockClusterTimescaleSpecs(overrides map[string]any) *serverv1.ClusterTimescaleSpecs {
	internal := false
	serviceType := "load-balancer"
	specs := &serverv1.ClusterTimescaleSpecs{
		TimescaleImage:               "timescale/timescaledb:latest-pg14",
		DatabaseName:                 "testdb",
		DatabaseReplicas:             1,
		Storage:                      "30Gi",
		Namespace:                    "default",
		ConnectionPoolReplicas:       1,
		ConnectionPoolMaxConnections: "100",
		ConnectionPoolSize:           "10",
		SecretName:                   "timescale-secret",
		BootstrapCloudResources:      false,
		Internal:                     &internal,
		ServiceType:                  &serviceType,
	}

	// Apply overrides
	for k, v := range overrides {
		switch k {
		case "storage":
			specs.Storage = v.(string)
		case "database_replicas":
			specs.DatabaseReplicas = v.(int32)
		case "connection_pool_size":
			specs.ConnectionPoolSize = v.(string)
		case "timescale_image":
			specs.TimescaleImage = v.(string)
		case "postgres_parameters":
			specs.PostgresParameters = v.(map[string]string)
		case "node_selector":
			specs.NodeSelector = v.(map[string]string)
		case "instance_type":
			specs.InstanceType = v.(string)
		case "nodepool":
			specs.Nodepool = v.(string)
		case "request":
			specs.Request = v.(*serverv1.KubeResourceConfig)
		case "gateway_port":
			port := v.(int32)
			specs.GatewayPort = &port
		case "gateway_id":
			id := v.(string)
			specs.GatewayId = &id
		}
	}

	return specs
}

// TestClusterTimescaleResourceCreate verifies that CreateClusterTimescaleDB is called with correct specs.
func TestClusterTimescaleResourceCreate(t *testing.T) {
	server := setupMockBuilderServer(t)

	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id    = "test-env-id"
  storage           = "30Gi"
  database_replicas = 1
  timescale_image   = "timescale/timescaledb:latest-pg14"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_timescale.test", "id", "test-cluster-id"),
					resource.TestCheckResourceAttr("chalk_cluster_timescale.test", "environment_id", "test-env-id"),
					resource.TestCheckResourceAttr("chalk_cluster_timescale.test", "storage", "30Gi"),
					resource.TestCheckResourceAttr("chalk_cluster_timescale.test", "database_replicas", "1"),
					func(s *terraform.State) error {
						// Verify CreateClusterTimescaleDB was called
						captured := server.GetCapturedRequests("CreateClusterTimescaleDB")
						require.Len(t, captured, 1, "Expected exactly one CreateClusterTimescaleDB call")

						req := captured[0].(*serverv1.CreateClusterTimescaleDBRequest)
						assert.Equal(t, []string{"test-env-id"}, req.EnvironmentIds)
						assert.Equal(t, "30Gi", req.Specs.Storage)
						assert.Equal(t, int32(1), req.Specs.DatabaseReplicas)
						assert.Equal(t, "timescale/timescaledb:latest-pg14", req.Specs.TimescaleImage)

						return nil
					},
				),
			},
		},
	})
}

// TestClusterTimescaleResourceUpdate verifies that UpdateClusterTimescaleDB is called for updates.
func TestClusterTimescaleResourceUpdate(t *testing.T) {
	server := setupMockBuilderServer(t)

	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id    = "test-env-id"
  storage           = "30Gi"
  database_replicas = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_timescale.test", "storage", "30Gi"),
				),
			},
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id    = "test-env-id"
  storage           = "50Gi"
  database_replicas = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_timescale.test", "storage", "50Gi"),
					func(s *terraform.State) error {
						// Verify UpdateClusterTimescaleDB was called
						captured := server.GetCapturedRequests("UpdateClusterTimescaleDB")
						require.NotEmpty(t, captured, "Expected at least one UpdateClusterTimescaleDB call")

						return nil
					},
				),
			},
		},
	})
}

// TestClusterTimescaleResourceUpdateFieldMask verifies that field mask contains only changed fields.
func TestClusterTimescaleResourceUpdateFieldMask(t *testing.T) {
	server := setupMockBuilderServer(t)

	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id    = "test-env-id"
  storage           = "30Gi"
  database_replicas = 1
}
`,
			},
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id    = "test-env-id"
  storage           = "50Gi"
  database_replicas = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						// Verify UpdateClusterTimescaleDB was called with correct field mask
						captured := server.GetCapturedRequests("UpdateClusterTimescaleDB")
						require.NotEmpty(t, captured, "Expected at least one UpdateClusterTimescaleDB call")

						req := captured[len(captured)-1].(*serverv1.UpdateClusterTimescaleDBRequest)
						assert.NotNil(t, req.UpdateMask, "Expected UpdateMask to be set")
						assert.Equal(t, []string{"storage"}, req.UpdateMask.Paths, "Expected only 'storage' in field mask")
						assert.Equal(t, "50Gi", req.Specs.Storage)

						return nil
					},
				),
			},
		},
	})
}

// TestClusterTimescaleResourceUpdateMultipleFields verifies that field mask includes all changed fields.
func TestClusterTimescaleResourceUpdateMultipleFields(t *testing.T) {
	server := setupMockBuilderServer(t)

	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id         = "test-env-id"
  storage                = "30Gi"
  database_replicas      = 1
  connection_pool_size   = "10"
}
`,
			},
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id         = "test-env-id"
  storage                = "50Gi"
  database_replicas      = 2
  connection_pool_size   = "100"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						// Verify UpdateClusterTimescaleDB was called with all changed fields
						captured := server.GetCapturedRequests("UpdateClusterTimescaleDB")
						require.NotEmpty(t, captured, "Expected at least one UpdateClusterTimescaleDB call")

						req := captured[len(captured)-1].(*serverv1.UpdateClusterTimescaleDBRequest)
						assert.NotNil(t, req.UpdateMask, "Expected UpdateMask to be set")

						expectedFields := []string{"storage", "database_replicas", "connection_pool_size"}
						assert.ElementsMatch(t, expectedFields, req.UpdateMask.Paths,
							"Expected all changed fields in field mask")

						assert.Equal(t, "50Gi", req.Specs.Storage)
						assert.Equal(t, int32(2), req.Specs.DatabaseReplicas)
						assert.Equal(t, "100", req.Specs.ConnectionPoolSize)

						return nil
					},
				),
			},
		},
	})
}

// TestClusterTimescaleResourceNoOpUpdate verifies no RPC call when no fields change.
func TestClusterTimescaleResourceNoOpUpdate(t *testing.T) {
	server := setupMockBuilderServer(t)

	// Configure Create response
	server.OnCreateClusterTimescaleDB().Return(&serverv1.CreateClusterTimescaleDBResponse{
		ClusterTimescaleId: "test-cluster-id",
		Specs: mockClusterTimescaleSpecs(map[string]any{
			"storage":           "30Gi",
			"database_replicas": int32(1),
		}),
	})

	// Configure Update response (should not be called)
	server.OnUpdateClusterTimescaleDB().Return(&serverv1.UpdateClusterTimescaleDBResponse{
		ClusterTimescaleId: "test-cluster-id",
		Specs: mockClusterTimescaleSpecs(map[string]any{
			"storage":           "30Gi",
			"database_replicas": int32(1),
		}),
	})

	setupTestEnv(t, server.URL)

	config := `
resource "chalk_cluster_timescale" "test" {
  environment_id    = "test-env-id"
  storage           = "30Gi"
  database_replicas = 1
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: config,
			},
			{
				Config: config, // Same config, no changes
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						// Verify UpdateClusterTimescaleDB was NOT called
						// We expect only 1 CreateClusterTimescaleDB call
						createCalls := server.GetCapturedRequests("CreateClusterTimescaleDB")
						updateCalls := server.GetCapturedRequests("UpdateClusterTimescaleDB")

						assert.Len(t, createCalls, 1, "Expected exactly one CreateClusterTimescaleDB call")
						assert.Empty(t, updateCalls, "Expected no UpdateClusterTimescaleDB calls for no-op update")

						return nil
					},
				),
			},
		},
	})
}

// TestClusterTimescaleResourceUpdateMapFields verifies map field updates trigger correct field mask.
func TestClusterTimescaleResourceUpdateMapFields(t *testing.T) {
	server := setupMockBuilderServer(t)

	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id = "test-env-id"
  postgres_parameters = {
    max_connections = "200"
  }
}
`,
			},
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id = "test-env-id"
  postgres_parameters = {
    max_connections = "300"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						// Verify UpdateClusterTimescaleDB was called with postgres_parameters in field mask
						captured := server.GetCapturedRequests("UpdateClusterTimescaleDB")
						require.NotEmpty(t, captured, "Expected at least one UpdateClusterTimescaleDB call")

						req := captured[len(captured)-1].(*serverv1.UpdateClusterTimescaleDBRequest)
						assert.NotNil(t, req.UpdateMask, "Expected UpdateMask to be set")
						assert.Contains(t, req.UpdateMask.Paths, "postgres_parameters")
						assert.Equal(t, "300", req.Specs.PostgresParameters["max_connections"])

						return nil
					},
				),
			},
		},
	})
}

// TestClusterTimescaleResourceUpdateObjectField verifies object field updates trigger correct field mask.
func TestClusterTimescaleResourceUpdateObjectField(t *testing.T) {
	server := setupMockBuilderServer(t)

	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id = "test-env-id"
  request = {
    cpu    = "500m"
    memory = "2Gi"
  }
}
`,
			},
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id = "test-env-id"
  request = {
    cpu    = "1000m"
    memory = "2Gi"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						// Verify UpdateClusterTimescaleDB was called with request in field mask
						captured := server.GetCapturedRequests("UpdateClusterTimescaleDB")
						require.NotEmpty(t, captured, "Expected at least one UpdateClusterTimescaleDB call")

						req := captured[len(captured)-1].(*serverv1.UpdateClusterTimescaleDBRequest)
						assert.NotNil(t, req.UpdateMask, "Expected UpdateMask to be set")
						assert.Contains(t, req.UpdateMask.Paths, "request")
						assert.Equal(t, "1000m", req.Specs.Request.Cpu)
						assert.Equal(t, "2Gi", req.Specs.Request.Memory)

						return nil
					},
				),
			},
		},
	})
}

// TestClusterTimescaleResourceCreateError verifies proper error handling when Create RPC fails.
func TestClusterTimescaleResourceCreateError(t *testing.T) {
	server := setupMockBuilderServer(t)
	setupTestEnv(t, server.URL)

	// Reset and configure Create to return an error
	server.Reset()
	server.OnCreateClusterTimescaleDB().ReturnError(
		connect.NewError(connect.CodeInvalidArgument, errors.New("invalid storage size")))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id    = "test-env-id"
  storage           = "invalid"
  database_replicas = 1
}
`,
				ExpectError: regexp.MustCompile("invalid storage size"),
			},
		},
	})
}

// TestClusterTimescaleResourceUpdateError verifies proper error handling when Update RPC fails.
func TestClusterTimescaleResourceUpdateError(t *testing.T) {
	server := setupMockBuilderServer(t)
	setupTestEnv(t, server.URL)

	// Reset and configure responses - Create succeeds, Update fails
	server.Reset()
	server.OnCreateClusterTimescaleDB().Return(&serverv1.CreateClusterTimescaleDBResponse{
		ClusterTimescaleId: "test-cluster-id",
		Specs: mockClusterTimescaleSpecs(map[string]any{
			"storage":           "30Gi",
			"database_replicas": int32(1),
		}),
	})
	server.OnGetClusterTimescaleDB().Return(&serverv1.GetClusterTimescaleDBResponse{
		Id: "test-cluster-id",
		Specs: mockClusterTimescaleSpecs(map[string]any{
			"storage":           "30Gi",
			"database_replicas": int32(1),
		}),
	})
	server.OnUpdateClusterTimescaleDB().ReturnError(
		connect.NewError(connect.CodeResourceExhausted, errors.New("quota exceeded")))
	// Configure Delete handler so destroy step works after update error
	server.OnDeleteClusterTimescaleDB().Return(&serverv1.DeleteClusterTimescaleDBResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id    = "test-env-id"
  storage           = "30Gi"
  database_replicas = 1
}
`,
			},
			{
				Config: `
resource "chalk_cluster_timescale" "test" {
  environment_id    = "test-env-id"
  storage           = "100Gi"
  database_replicas = 1
}
`,
				ExpectError: regexp.MustCompile("quota exceeded"),
			},
		},
	})
}
