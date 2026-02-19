package provider

import (
	"errors"
	"regexp"
	"strings"
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

// literalVal extracts literal values from a proto Config map, for use in test assertions.
func literalVal(config map[string]*serverv1.IntegrationConfigValue, key string) string {
	v, ok := config[key]
	if !ok {
		return ""
	}
	lit, ok := v.Value.(*serverv1.IntegrationConfigValue_Literal)
	if !ok {
		return ""
	}
	return lit.Literal
}

// secretIdVal extracts secret_id values from a proto Config map, for use in test assertions.
func secretIdVal(config map[string]*serverv1.IntegrationConfigValue, key string) string {
	v, ok := config[key]
	if !ok {
		return ""
	}
	sid, ok := v.Value.(*serverv1.IntegrationConfigValue_SecretId)
	if !ok {
		return ""
	}
	return sid.SecretId
}

func setupMockIntegrationsServer(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	var currentIntegration *serverv1.Integration
	// Stores literal values keyed by config key, for use in GetIntegration/GetIntegrationValue.
	var currentLiterals map[string]string

	server.OnInsertIntegration().WithBehavior(func(req proto.Message) (proto.Message, error) {
		insertReq := req.(*serverv1.InsertIntegrationRequest)
		currentLiterals = make(map[string]string)
		for k, v := range insertReq.Config {
			if lit, ok := v.Value.(*serverv1.IntegrationConfigValue_Literal); ok {
				currentLiterals[k] = lit.Literal
			}
		}
		name := insertReq.Name
		currentIntegration = &serverv1.Integration{
			Id:            "test-integration-id",
			Name:          &name,
			Kind:          insertReq.IntegrationKind,
			EnvironmentId: "test-env-id",
		}
		return &serverv1.InsertIntegrationResponse{
			Integration: currentIntegration,
		}, nil
	})

	server.OnGetIntegration().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if currentIntegration == nil {
			return &serverv1.GetIntegrationResponse{}, nil
		}
		// Simulate real server behavior: withhold secrets whose keys contain
		// "PASSWORD" — those must be fetched via GetIntegrationValue.
		secrets := make([]*serverv1.SecretWithValue, 0, len(currentLiterals))
		for k, v := range currentLiterals {
			if strings.Contains(k, "PASSWORD") {
				continue
			}
			val := v
			secrets = append(secrets, &serverv1.SecretWithValue{
				Name:  k,
				Value: &val,
			})
		}
		return &serverv1.GetIntegrationResponse{
			IntegrationWithSecrets: &serverv1.IntegrationWithSecrets{
				Integration: currentIntegration,
				Secrets:     secrets,
			},
		}, nil
	})

	server.OnGetIntegrationValue().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getReq := req.(*serverv1.GetIntegrationValueRequest)
		val, ok := currentLiterals[getReq.SecretName]
		if !ok {
			return &serverv1.GetIntegrationValueResponse{}, nil
		}
		return &serverv1.GetIntegrationValueResponse{
			Secretvalue: &serverv1.SecretValue{
				Name:  getReq.SecretName,
				Value: val,
			},
		}, nil
	})

	server.OnUpdateIntegration().WithBehavior(func(req proto.Message) (proto.Message, error) {
		updateReq := req.(*serverv1.UpdateIntegrationRequest)
		currentLiterals = make(map[string]string)
		for k, v := range updateReq.Config {
			if lit, ok := v.Value.(*serverv1.IntegrationConfigValue_Literal); ok {
				currentLiterals[k] = lit.Literal
			}
		}
		if currentIntegration != nil {
			name := updateReq.Name
			currentIntegration.Name = &name
		}
		return &serverv1.UpdateIntegrationResponse{
			Integration: currentIntegration,
		}, nil
	})

	server.OnDeleteIntegration().Return(&serverv1.DeleteIntegrationResponse{})

	return server
}

func TestDatasourceResourceCreate(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource" "test" {
  name           = "my-postgres"
  kind           = "postgresql"
  environment_id = "test-env-id"

  config = {
    PGHOST     = { literal = "db.example.com" }
    PGPORT     = { literal = "5432" }
    PGDATABASE = { literal = "mydb" }
    PGUSER     = { literal = "admin" }
    PGPASSWORD = { literal = "secret" }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource.test", "id", "test-integration-id"),
					resource.TestCheckResourceAttr("chalk_datasource.test", "name", "my-postgres"),
					resource.TestCheckResourceAttr("chalk_datasource.test", "kind", "postgresql"),
					resource.TestCheckResourceAttr("chalk_datasource.test", "environment_id", "test-env-id"),
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

func TestDatasourceResourceCreateWithSecretId(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource" "test" {
  name           = "my-postgres"
  kind           = "postgresql"
  environment_id = "test-env-id"

  config = {
    PGHOST     = { literal   = "db.example.com" }
    PGPASSWORD = { secret_id = "my-pg-password-secret" }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1, "Expected exactly one InsertIntegration call")

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "db.example.com", literalVal(req.Config, "PGHOST"))
						assert.Equal(t, "my-pg-password-secret", secretIdVal(req.Config, "PGPASSWORD"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceResourceUpdate(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource" "test" {
  name           = "my-postgres"
  kind           = "postgresql"
  environment_id = "test-env-id"

  config = {
    PGHOST = { literal = "db.example.com" }
    PGPORT = { literal = "5432" }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource.test", "name", "my-postgres"),
				),
			},
			{
				Config: `
resource "chalk_datasource" "test" {
  name           = "my-postgres-updated"
  kind           = "postgresql"
  environment_id = "test-env-id"

  config = {
    PGHOST = { literal = "db2.example.com" }
    PGPORT = { literal = "5433" }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource.test", "name", "my-postgres-updated"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateIntegration")
						require.NotEmpty(t, captured, "Expected at least one UpdateIntegration call")

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

func TestDatasourceResourceDelete(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource" "test" {
  name           = "my-postgres"
  kind           = "postgresql"
  environment_id = "test-env-id"

  config = {
    PGHOST = { literal = "db.example.com" }
  }
}
`,
			},
		},
		CheckDestroy: func(s *terraform.State) error {
			captured := server.GetCapturedRequests("DeleteIntegration")
			require.NotEmpty(t, captured, "Expected at least one DeleteIntegration call")

			req := captured[len(captured)-1].(*serverv1.DeleteIntegrationRequest)
			assert.Equal(t, "test-integration-id", req.Id)

			return nil
		},
	})
}

func TestDatasourceResourceImport(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource" "test" {
  name           = "my-postgres"
  kind           = "postgresql"
  environment_id = "test-env-id"

  config = {
    PGHOST = { literal = "db.example.com" }
    PGPORT = { literal = "5432" }
  }
}
`,
			},
			{
				ResourceName:            "chalk_datasource.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config"},
				ImportStateId:           "test-env-id/test-integration-id",
			},
		},
	})
}

func TestDatasourceResourceImportInvalidID(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	cfg := `
resource "chalk_datasource" "test" {
  name           = "my-postgres"
  kind           = "postgresql"
  environment_id = "test-env-id"

  config = {
    PGHOST = { literal = "db.example.com" }
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config:        cfg,
				ResourceName:  "chalk_datasource.test",
				ImportState:   true,
				ImportStateId: "just-an-id-without-env",
				ExpectError:   regexp.MustCompile("Invalid Import ID"),
			},
		},
	})
}

func TestDatasourceResourceConfigBothSet(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource" "test" {
  name           = "my-postgres"
  kind           = "postgresql"
  environment_id = "test-env-id"

  config = {
    PGHOST = { literal = "db.example.com", secret_id = "some-secret" }
  }
}
`,
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}

func TestDatasourceResourceConfigNeitherSet(t *testing.T) {
	server := setupMockIntegrationsServer(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource" "test" {
  name           = "my-postgres"
  kind           = "postgresql"
  environment_id = "test-env-id"

  config = {
    PGHOST = {}
  }
}
`,
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}

func TestDatasourceResourceCreateError(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnInsertIntegration().ReturnError(
		connect.NewError(connect.CodeInvalidArgument, errors.New("invalid datasource configuration")))

	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_datasource" "test" {
  name           = "bad-datasource"
  kind           = "postgresql"
  environment_id = "test-env-id"

  config = {
    PGHOST = { literal = "db.example.com" }
  }
}
`,
				ExpectError: regexp.MustCompile("invalid datasource"),
			},
		},
	})
}
