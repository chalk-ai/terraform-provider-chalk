package provider

import (
	"regexp"
	"testing"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestDatasourceDatabricksResourceCreate(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_databricks" "test" {
  name           = "my-databricks"
  environment_id = "test-env-id"

  host        = { literal = "my-workspace.cloud.databricks.com" }
  http_path   = { literal = "/sql/1.0/endpoints/abc123" }
  access_token = { literal = "dapi123456" }
  database    = { literal = "default" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_databricks.test", "id", "test-integration-id"),
					resource.TestCheckResourceAttr("chalk_datasource_databricks.test", "name", "my-databricks"),
					resource.TestCheckResourceAttr("chalk_datasource_databricks.test", "environment_id", "test-env-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1, "Expected exactly one InsertIntegration call")

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "my-databricks", req.Name)
						assert.Equal(t, serverv1.IntegrationKind_INTEGRATION_KIND_DATABRICKS, req.IntegrationKind)
						assert.Equal(t, "my-workspace.cloud.databricks.com", literalVal(req.Config, "DATABRICKS_HOST"))
						assert.Equal(t, "/sql/1.0/endpoints/abc123", literalVal(req.Config, "DATABRICKS_HTTP_PATH"))
						assert.Equal(t, "dapi123456", literalVal(req.Config, "DATABRICKS_TOKEN"))
						assert.Equal(t, "default", literalVal(req.Config, "DATABRICKS_DATABASE"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceDatabricksResourceCreateMinimal(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_databricks" "test" {
  name           = "my-databricks"
  environment_id = "test-env-id"

  host = { literal = "my-workspace.cloud.databricks.com" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_databricks.test", "id", "test-integration-id"),
					resource.TestCheckNoResourceAttr("chalk_datasource_databricks.test", "access_token"),
					resource.TestCheckNoResourceAttr("chalk_datasource_databricks.test", "database"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "my-workspace.cloud.databricks.com", literalVal(req.Config, "DATABRICKS_HOST"))
						_, hasToken := req.Config["DATABRICKS_TOKEN"]
						assert.False(t, hasToken, "DATABRICKS_TOKEN should not be in config when not set")

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceDatabricksResourceCreateWithOAuth(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_databricks" "test" {
  name           = "my-databricks"
  environment_id = "test-env-id"

  host          = { literal = "my-workspace.cloud.databricks.com" }
  http_path     = { literal = "/sql/1.0/endpoints/abc123" }
  client_id     = { literal = "oauth-client-id" }
  client_secret = { secret_id = "databricks-client-secret" }
  database      = { literal = "default" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_databricks.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "oauth-client-id", literalVal(req.Config, "DATABRICKS_CLIENT_ID"))
						assert.Equal(t, "databricks-client-secret", secretIdVal(req.Config, "DATABRICKS_CLIENT_SECRET"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceDatabricksResourceCreateWithExtraConfig(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_databricks" "test" {
  name           = "my-databricks"
  environment_id = "test-env-id"

  host      = { literal = "my-workspace.cloud.databricks.com" }
  http_path = { literal = "/sql/1.0/endpoints/abc123" }

  config = {
    ENGINE_ARGUMENTS = { literal = "{\"timeout\": 30}" }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_databricks.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "my-workspace.cloud.databricks.com", literalVal(req.Config, "DATABRICKS_HOST"))
						assert.Equal(t, "{\"timeout\": 30}", literalVal(req.Config, "ENGINE_ARGUMENTS"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceDatabricksResourceUpdate(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_databricks" "test" {
  name           = "my-databricks"
  environment_id = "test-env-id"

  host = { literal = "my-workspace.cloud.databricks.com" }
}
`,
				Check: resource.TestCheckResourceAttr("chalk_datasource_databricks.test", "name", "my-databricks"),
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_databricks" "test" {
  name           = "my-databricks-updated"
  environment_id = "test-env-id"

  host     = { literal = "my-workspace2.cloud.databricks.com" }
  database = { literal = "production" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_databricks.test", "name", "my-databricks-updated"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateIntegration")
						require.NotEmpty(t, captured)

						req := captured[len(captured)-1].(*serverv1.UpdateIntegrationRequest)
						assert.Equal(t, "my-databricks-updated", req.Name)
						assert.Equal(t, "test-integration-id", req.IntegrationId)
						assert.Equal(t, "my-workspace2.cloud.databricks.com", literalVal(req.Config, "DATABRICKS_HOST"))
						assert.Equal(t, "production", literalVal(req.Config, "DATABRICKS_DATABASE"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceDatabricksResourceDelete(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_databricks" "test" {
  name           = "my-databricks"
  environment_id = "test-env-id"

  host = { literal = "my-workspace.cloud.databricks.com" }
}
`,
			},
		},
		CheckDestroy: func(s *terraform.State) error {
			captured := server.GetCapturedRequests("DeleteIntegration")
			require.NotEmpty(t, captured)

			req := captured[len(captured)-1].(*serverv1.DeleteIntegrationRequest)
			assert.Equal(t, "test-integration-id", req.Id)

			return nil
		},
	})
}

func TestDatasourceDatabricksResourceImport(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_databricks" "test" {
  name           = "my-databricks"
  environment_id = "test-env-id"

  host      = { literal = "my-workspace.cloud.databricks.com" }
  http_path = { literal = "/sql/1.0/endpoints/abc123" }
}
`,
			},
			{
				ResourceName:      "chalk_datasource_databricks.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "test-env-id/test-integration-id",
			},
		},
	})
}

func TestDatasourceDatabricksResourceImportWrongKind(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnInsertIntegration().WithBehavior(func(req proto.Message) (proto.Message, error) {
		insertReq := req.(*serverv1.InsertIntegrationRequest)
		name := insertReq.Name
		return &serverv1.InsertIntegrationResponse{
			Integration: &serverv1.Integration{
				Id:            "test-integration-id",
				Name:          &name,
				Kind:          serverv1.IntegrationKind_INTEGRATION_KIND_DATABRICKS,
				EnvironmentId: "test-env-id",
			},
		}, nil
	})

	snowflakeName := "my-snowflake"
	server.OnGetIntegration().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.GetIntegrationResponse{
			IntegrationWithSecrets: &serverv1.IntegrationWithSecrets{
				Integration: &serverv1.Integration{
					Id:            "test-integration-id",
					Name:          &snowflakeName,
					Kind:          serverv1.IntegrationKind_INTEGRATION_KIND_SNOWFLAKE,
					EnvironmentId: "test-env-id",
				},
			},
		}, nil
	})
	server.OnDeleteIntegration().Return(&serverv1.DeleteIntegrationResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_databricks" "test" {
  name           = "my-databricks"
  environment_id = "test-env-id"

  host = { literal = "my-workspace.cloud.databricks.com" }
}
`,
				ExpectError: regexp.MustCompile("Unexpected Integration Kind"),
			},
		},
	})
}

func TestDatasourceDatabricksResourceConfigConflict(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_databricks" "test" {
  name           = "my-databricks"
  environment_id = "test-env-id"

  host = { literal = "my-workspace.cloud.databricks.com" }

  config = {
    DATABRICKS_HOST = { literal = "other.databricks.com" }
  }
}
`,
				ExpectError: regexp.MustCompile("Reserved Config Key"),
			},
		},
	})
}
