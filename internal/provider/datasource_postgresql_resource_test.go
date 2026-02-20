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

func TestDatasourcePostgresqlResourceCreate(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres"
  environment_id = "test-env-id"

  host     = { literal = "db.example.com" }
  port     = { literal = "5432" }
  database = { literal = "mydb" }
  user     = { literal = "admin" }
  password = { literal = "secret" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_postgresql.test", "id", "test-integration-id"),
					resource.TestCheckResourceAttr("chalk_datasource_postgresql.test", "name", "my-postgres"),
					resource.TestCheckResourceAttr("chalk_datasource_postgresql.test", "environment_id", "test-env-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1, "Expected exactly one InsertIntegration call")

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "my-postgres", req.Name)
						assert.Equal(t, serverv1.IntegrationKind_INTEGRATION_KIND_POSTGRESQL, req.IntegrationKind)
						assert.Equal(t, "db.example.com", literalVal(req.Config, "PGHOST"))
						assert.Equal(t, "5432", literalVal(req.Config, "PGPORT"))
						assert.Equal(t, "mydb", literalVal(req.Config, "PGDATABASE"))
						assert.Equal(t, "admin", literalVal(req.Config, "PGUSER"))
						assert.Equal(t, "secret", literalVal(req.Config, "PGPASSWORD"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourcePostgresqlResourceCreateWithoutPassword(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres"
  environment_id = "test-env-id"

  host     = { literal = "db.example.com" }
  port     = { literal = "5432" }
  database = { literal = "mydb" }
  user     = { literal = "admin" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_postgresql.test", "id", "test-integration-id"),
					resource.TestCheckNoResourceAttr("chalk_datasource_postgresql.test", "password"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "db.example.com", literalVal(req.Config, "PGHOST"))
						_, hasPGPassword := req.Config["PGPASSWORD"]
						assert.False(t, hasPGPassword, "PGPASSWORD should not be in config when password is not set")

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourcePostgresqlResourceCreateWithSecretId(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres"
  environment_id = "test-env-id"

  host     = { literal   = "db.example.com" }
  port     = { literal   = "5432" }
  database = { literal   = "mydb" }
  user     = { literal   = "admin" }
  password = { secret_id = "pg-password-secret" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_postgresql.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "pg-password-secret", secretIdVal(req.Config, "PGPASSWORD"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourcePostgresqlResourceCreateWithExtraConfig(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres"
  environment_id = "test-env-id"

  host     = { literal = "db.example.com" }
  port     = { literal = "5432" }
  database = { literal = "mydb" }
  user     = { literal = "admin" }

  config = {
    PGSSLMODE = { literal = "require" }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_postgresql.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "db.example.com", literalVal(req.Config, "PGHOST"))
						assert.Equal(t, "require", literalVal(req.Config, "PGSSLMODE"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourcePostgresqlResourceUpdate(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres"
  environment_id = "test-env-id"

  host     = { literal = "db.example.com" }
  port     = { literal = "5432" }
  database = { literal = "mydb" }
  user     = { literal = "admin" }
}
`,
				Check: resource.TestCheckResourceAttr("chalk_datasource_postgresql.test", "name", "my-postgres"),
			},
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres-updated"
  environment_id = "test-env-id"

  host     = { literal = "db2.example.com" }
  port     = { literal = "5433" }
  database = { literal = "mydb" }
  user     = { literal = "admin" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_postgresql.test", "name", "my-postgres-updated"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateIntegration")
						require.NotEmpty(t, captured)

						req := captured[len(captured)-1].(*serverv1.UpdateIntegrationRequest)
						assert.Equal(t, "my-postgres-updated", req.Name)
						assert.Equal(t, "test-integration-id", req.IntegrationId)
						assert.Equal(t, "db2.example.com", literalVal(req.Config, "PGHOST"))
						assert.Equal(t, "5433", literalVal(req.Config, "PGPORT"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourcePostgresqlResourceDelete(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres"
  environment_id = "test-env-id"

  host     = { literal = "db.example.com" }
  port     = { literal = "5432" }
  database = { literal = "mydb" }
  user     = { literal = "admin" }
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

func TestDatasourcePostgresqlResourceImport(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres"
  environment_id = "test-env-id"

  host     = { literal = "db.example.com" }
  port     = { literal = "5432" }
  database = { literal = "mydb" }
  user     = { literal = "admin" }
}
`,
			},
			{
				ResourceName:      "chalk_datasource_postgresql.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "test-env-id/test-integration-id",
			},
		},
	})
}

func TestDatasourcePostgresqlResourceImportWithExtraConfig(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres"
  environment_id = "test-env-id"

  host     = { literal = "db.example.com" }
  port     = { literal = "5432" }
  database = { literal = "mydb" }
  user     = { literal = "admin" }

  config = {
    PGSSLMODE = { literal = "require" }
  }
}
`,
			},
			{
				ResourceName:      "chalk_datasource_postgresql.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "test-env-id/test-integration-id",
			},
		},
	})
}

func TestDatasourcePostgresqlResourceImportWrongKind(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	// Create succeeds and returns a PostgreSQL integration ID.
	server.OnInsertIntegration().WithBehavior(func(req proto.Message) (proto.Message, error) {
		insertReq := req.(*serverv1.InsertIntegrationRequest)
		name := insertReq.Name
		return &serverv1.InsertIntegrationResponse{
			Integration: &serverv1.Integration{
				Id:            "test-integration-id",
				Name:          &name,
				Kind:          serverv1.IntegrationKind_INTEGRATION_KIND_POSTGRESQL,
				EnvironmentId: "test-env-id",
			},
		}, nil
	})

	// Subsequent reads return Snowflake — simulates importing the wrong resource.
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

	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres"
  environment_id = "test-env-id"

  host     = { literal = "db.example.com" }
  port     = { literal = "5432" }
  database = { literal = "mydb" }
  user     = { literal = "admin" }
}
`,
				ExpectError: regexp.MustCompile("Unexpected Integration Kind"),
			},
		},
	})
}

func TestDatasourcePostgresqlResourceConfigConflict(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource_postgresql" "test" {
  name           = "my-postgres"
  environment_id = "test-env-id"

  host     = { literal = "db.example.com" }
  port     = { literal = "5432" }
  database = { literal = "mydb" }
  user     = { literal = "admin" }

  config = {
    PGHOST = { literal = "other.example.com" }
  }
}
`,
				ExpectError: regexp.MustCompile("Reserved Config Key"),
			},
		},
	})
}
