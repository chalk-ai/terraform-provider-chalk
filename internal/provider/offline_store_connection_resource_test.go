package provider

import (
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func bigqueryConnection(name string) *serverv1.OfflineStoreConnection {
	return &serverv1.OfflineStoreConnection{
		Id:            "test-connection-id",
		EnvironmentId: "test-environment-id",
		Name:          name,
		Config: &serverv1.OfflineStoreConnectionConfigStored{
			Config: &serverv1.OfflineStoreConnectionConfigStored_Bigquery{
				Bigquery: &serverv1.BigQueryOfflineStoreConnectionConfig{
					ProjectId: "my-project",
					DatasetId: "my-dataset",
				},
			},
		},
	}
}

func setupMockServerOfflineStoreConnection(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateOfflineStoreConnection().Return(&serverv1.CreateOfflineStoreConnectionResponse{
		Connection: bigqueryConnection("test-connection"),
	})
	server.OnGetOfflineStoreConnection().Return(&serverv1.GetOfflineStoreConnectionResponse{
		Connection: bigqueryConnection("test-connection"),
	})
	server.OnDeleteOfflineStoreConnection().Return(&serverv1.DeleteOfflineStoreConnectionResponse{})

	return server
}

// TestOfflineStoreConnectionCreate verifies the basic create/read/delete lifecycle with BigQuery.
func TestOfflineStoreConnectionCreate(t *testing.T) {
	t.Parallel()
	server := setupMockServerOfflineStoreConnection(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_offline_store_connection" "test" {
  environment_id = "test-environment-id"
  name           = "test-connection"
  bigquery = {
    project_id = "my-project"
    dataset_id = "my-dataset"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "id", "test-connection-id"),
					resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "environment_id", "test-environment-id"),
					resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "name", "test-connection"),
					resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "bigquery.project_id", "my-project"),
					resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "bigquery.dataset_id", "my-dataset"),
				),
			},
		},
	})
}

// TestOfflineStoreConnectionUpdate verifies that a name/config change triggers an in-place update
// (not a replacement), and that the correct update_mask is sent to the server.
func TestOfflineStoreConnectionUpdate(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateOfflineStoreConnection().Return(&serverv1.CreateOfflineStoreConnectionResponse{
		Connection: bigqueryConnection("original-name"),
	})
	server.OnDeleteOfflineStoreConnection().Return(&serverv1.DeleteOfflineStoreConnectionResponse{})

	// Update mock: mirror the request so state stays consistent after apply.
	server.OnUpdateOfflineStoreConnection().WithBehavior(func(req proto.Message) (proto.Message, error) {
		updateReq := req.(*serverv1.UpdateOfflineStoreConnectionRequest)
		conn := &serverv1.OfflineStoreConnection{
			Id:            "test-connection-id",
			EnvironmentId: "test-environment-id",
			Name:          updateReq.Connection.Name,
			Config: &serverv1.OfflineStoreConnectionConfigStored{
				Config: &serverv1.OfflineStoreConnectionConfigStored_Bigquery{
					Bigquery: &serverv1.BigQueryOfflineStoreConnectionConfig{
						ProjectId: updateReq.Connection.Config.GetBigquery().ProjectId,
						DatasetId: updateReq.Connection.Config.GetBigquery().DatasetId,
					},
				},
			},
		}
		return &serverv1.UpdateOfflineStoreConnectionResponse{Connection: conn}, nil
	})

	// Get mock: return the last update's state, or the original if no updates yet.
	server.OnGetOfflineStoreConnection().WithBehavior(func(req proto.Message) (proto.Message, error) {
		updates := server.GetCapturedRequests("UpdateOfflineStoreConnection")
		if len(updates) == 0 {
			return &serverv1.GetOfflineStoreConnectionResponse{Connection: bigqueryConnection("original-name")}, nil
		}
		lastUpdate := updates[len(updates)-1].(*serverv1.UpdateOfflineStoreConnectionRequest)
		conn := &serverv1.OfflineStoreConnection{
			Id:            "test-connection-id",
			EnvironmentId: "test-environment-id",
			Name:          lastUpdate.Connection.Name,
			Config: &serverv1.OfflineStoreConnectionConfigStored{
				Config: &serverv1.OfflineStoreConnectionConfigStored_Bigquery{
					Bigquery: &serverv1.BigQueryOfflineStoreConnectionConfig{
						ProjectId: lastUpdate.Connection.Config.GetBigquery().ProjectId,
						DatasetId: lastUpdate.Connection.Config.GetBigquery().DatasetId,
					},
				},
			},
		}
		return &serverv1.GetOfflineStoreConnectionResponse{Connection: conn}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_offline_store_connection" "test" {
  environment_id = "test-environment-id"
  name           = "original-name"
  bigquery = {
    project_id = "my-project"
    dataset_id = "my-dataset"
  }
}
`,
				Check: resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "name", "original-name"),
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_offline_store_connection" "test" {
  environment_id = "test-environment-id"
  name           = "updated-name"
  bigquery = {
    project_id = "my-project"
    dataset_id = "my-dataset"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "name", "updated-name"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateOfflineStoreConnection")
						require.Len(t, captured, 1, "expected exactly one UpdateOfflineStoreConnection call")
						req := captured[0].(*serverv1.UpdateOfflineStoreConnectionRequest)
						assert.Equal(t, []string{"name", "config"}, req.UpdateMask.Paths)
						return nil
					},
				),
			},
			// Verify that changing config within the same type is an in-place update, not a replacement.
			{
				Config: providerConfig(server.URL) + `
resource "chalk_offline_store_connection" "test" {
  environment_id = "test-environment-id"
  name           = "updated-name"
  bigquery = {
    project_id = "my-project"
    dataset_id = "updated-dataset"
  }
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"chalk_offline_store_connection.test",
							plancheck.ResourceActionUpdate,
						),
					},
				},
			},
		},
	})
}

// TestOfflineStoreConnectionTypeChangeRequiresReplace verifies that switching the connection
// type (e.g. BigQuery → Snowflake) forces a destroy-before-create rather than an in-place update.
func TestOfflineStoreConnectionTypeChangeRequiresReplace(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	warehouse := "my-warehouse"
	database := "my-database"
	schemaStr := "my-schema"
	role := "my-role"

	snowflakeConn := &serverv1.OfflineStoreConnection{
		Id:            "test-snowflake-id",
		EnvironmentId: "test-environment-id",
		Name:          "test-connection",
		Config: &serverv1.OfflineStoreConnectionConfigStored{
			Config: &serverv1.OfflineStoreConnectionConfigStored_Snowflake{
				Snowflake: &serverv1.SnowflakeOfflineStoreConnectionConfigStored{
					Credentials: &serverv1.SnowflakeCredentialsStored{
						Account:          "my-account",
						Username:         "my-user",
						PasswordSecretId: proto.String("secret-id"),
						Warehouse:        &warehouse,
						Database:         &database,
						Schema:           &schemaStr,
						Role:             &role,
					},
				},
			},
		},
	}

	server.OnCreateOfflineStoreConnection().WithBehavior(func(req proto.Message) (proto.Message, error) {
		creates := server.GetCapturedRequests("CreateOfflineStoreConnection")
		// The testserver records the request before invoking the behavior, so len == 1 on the first call.
		if len(creates) == 1 {
			return &serverv1.CreateOfflineStoreConnectionResponse{Connection: bigqueryConnection("test-connection")}, nil
		}
		return &serverv1.CreateOfflineStoreConnectionResponse{Connection: snowflakeConn}, nil
	})

	// Get mock: return snowflake connection only after the second create (type switch).
	server.OnGetOfflineStoreConnection().WithBehavior(func(req proto.Message) (proto.Message, error) {
		creates := server.GetCapturedRequests("CreateOfflineStoreConnection")
		if len(creates) >= 2 {
			return &serverv1.GetOfflineStoreConnectionResponse{Connection: snowflakeConn}, nil
		}
		return &serverv1.GetOfflineStoreConnectionResponse{Connection: bigqueryConnection("test-connection")}, nil
	})

	server.OnDeleteOfflineStoreConnection().Return(&serverv1.DeleteOfflineStoreConnectionResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_offline_store_connection" "test" {
  environment_id = "test-environment-id"
  name           = "test-connection"
  bigquery = {
    project_id = "my-project"
    dataset_id = "my-dataset"
  }
}
`,
				Check: resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "id", "test-connection-id"),
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_offline_store_connection" "test" {
  environment_id = "test-environment-id"
  name           = "test-connection"
  snowflake = {
    credentials = {
      account   = "my-account"
      username  = "my-user"
      password  = "super-secret"
      warehouse = "my-warehouse"
      database  = "my-database"
      schema    = "my-schema"
      role      = "my-role"
    }
  }
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(
							"chalk_offline_store_connection.test",
							plancheck.ResourceActionDestroyBeforeCreate,
						),
					},
				},
				Check: resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "id", "test-snowflake-id"),
			},
		},
	})
}

// TestOfflineStoreConnectionReadNotFound verifies that a CodeNotFound on Get removes the resource from state.
func TestOfflineStoreConnectionReadNotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateOfflineStoreConnection().Return(&serverv1.CreateOfflineStoreConnectionResponse{
		Connection: bigqueryConnection("test-connection"),
	})
	server.OnDeleteOfflineStoreConnection().Return(&serverv1.DeleteOfflineStoreConnectionResponse{})

	var getCallCount int
	server.OnGetOfflineStoreConnection().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("connection not found"))
		}
		return &serverv1.GetOfflineStoreConnectionResponse{
			Connection: bigqueryConnection("test-connection"),
		}, nil
	})

	config := providerConfig(server.URL) + `
resource "chalk_offline_store_connection" "test" {
  environment_id = "test-environment-id"
  name           = "test-connection"
  bigquery = {
    project_id = "my-project"
    dataset_id = "my-dataset"
  }
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: config},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// snowflakeConn builds a stored Snowflake connection for test use.
// secretField is either "password_secret_id" or "private_key_secret_id".
func snowflakeStoredConn(secretField string) *serverv1.OfflineStoreConnection {
	warehouse := "my-warehouse"
	database := "my-database"
	schemaStr := "my-schema"
	role := "my-role"
	creds := &serverv1.SnowflakeCredentialsStored{
		Account:   "my-account",
		Username:  "my-user",
		Warehouse: &warehouse,
		Database:  &database,
		Schema:    &schemaStr,
		Role:      &role,
	}
	if secretField == "password_secret_id" {
		creds.PasswordSecretId = proto.String("secret-id-for-password")
	} else {
		creds.PrivateKeySecretId = proto.String("secret-id-for-private-key")
	}
	return &serverv1.OfflineStoreConnection{
		Id:            "test-snowflake-id",
		EnvironmentId: "test-environment-id",
		Name:          "test-snowflake",
		Config: &serverv1.OfflineStoreConnectionConfigStored{
			Config: &serverv1.OfflineStoreConnectionConfigStored_Snowflake{
				Snowflake: &serverv1.SnowflakeOfflineStoreConnectionConfigStored{
					Credentials: creds,
				},
			},
		},
	}
}

// TestOfflineStoreConnectionSnowflakeSensitivePreserved verifies that sensitive credential
// fields (password/private_key) are preserved from state on refresh, since the server only
// returns secret IDs. Subtests cover both auth methods.
func TestOfflineStoreConnectionSnowflakeSensitivePreserved(t *testing.T) {
	tests := []struct {
		name        string
		configField string // "password" or "private_key"
		secretField string // used to build server response
		stateAttr   string // terraform attribute path to check
	}{
		{
			name:        "password",
			configField: "password",
			secretField: "password_secret_id",
			stateAttr:   "snowflake.credentials.password",
		},
		{
			name:        "private_key",
			configField: "private_key",
			secretField: "private_key_secret_id",
			stateAttr:   "snowflake.credentials.private_key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := testserver.NewMockBuilderServer(t)
			t.Cleanup(func() { server.Close() })

			conn := snowflakeStoredConn(tc.secretField)
			server.OnCreateOfflineStoreConnection().Return(&serverv1.CreateOfflineStoreConnectionResponse{Connection: conn})
			server.OnGetOfflineStoreConnection().Return(&serverv1.GetOfflineStoreConnectionResponse{Connection: conn})
			server.OnDeleteOfflineStoreConnection().Return(&serverv1.DeleteOfflineStoreConnectionResponse{})

			config := providerConfig(server.URL) + fmt.Sprintf(`
resource "chalk_offline_store_connection" "test" {
  environment_id = "test-environment-id"
  name           = "test-snowflake"
  snowflake = {
    credentials = {
      account    = "my-account"
      username   = "my-user"
      %s         = "super-secret"
      warehouse  = "my-warehouse"
      database   = "my-database"
      schema     = "my-schema"
      role       = "my-role"
    }
  }
}
`, tc.configField)

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
				Steps: []resource.TestStep{
					{
						Config: config,
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "id", "test-snowflake-id"),
							// Sensitive field is preserved from plan/state, not overwritten by server response
							resource.TestCheckResourceAttr("chalk_offline_store_connection.test", tc.stateAttr, "super-secret"),
							resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "snowflake.credentials.account", "my-account"),
							resource.TestCheckResourceAttr("chalk_offline_store_connection.test", "snowflake.credentials.username", "my-user"),
						),
					},
					// Verify sensitive field survives a standalone refresh.
					{
						RefreshState: true,
						Check:        resource.TestCheckResourceAttr("chalk_offline_store_connection.test", tc.stateAttr, "super-secret"),
					},
				},
			})
		})
	}
}
