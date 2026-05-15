package provider

import (
	"errors"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const testOfflineStoreConnectionDataSourceConfig = `
data "chalk_offline_store_connection" "test" {
  environment_id = "test-env-id"
  id             = "test-conn-id"
}
`

func TestOfflineStoreConnectionDataSource_Snowflake(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	warehouse := "MY_WH"
	database := "MY_DB"
	schemaName := "PUBLIC"
	role := "MY_ROLE"

	server.OnGetOfflineStoreConnection().Return(&serverv1.GetOfflineStoreConnectionResponse{
		Connection: &serverv1.OfflineStoreConnection{
			Id:            "test-conn-id",
			Name:          "snow-conn",
			EnvironmentId: "test-env-id",
			Config: &serverv1.OfflineStoreConnectionConfigStored{
				Config: &serverv1.OfflineStoreConnectionConfigStored_Snowflake{
					Snowflake: &serverv1.SnowflakeOfflineStoreConnectionConfigStored{
						Credentials: &serverv1.SnowflakeCredentialsStored{
							Account:   "myaccount.us-east-1",
							Username:  "service-account",
							Warehouse: &warehouse,
							Database:  &database,
							Schema:    &schemaName,
							Role:      &role,
						},
					},
				},
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testOfflineStoreConnectionDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "id", "test-conn-id"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "name", "snow-conn"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "environment_id", "test-env-id"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "kind", "snowflake"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "snowflake.account", "myaccount.us-east-1"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "snowflake.username", "service-account"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "snowflake.warehouse", "MY_WH"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "snowflake.database", "MY_DB"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "snowflake.schema", "PUBLIC"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "snowflake.role", "MY_ROLE"),
				),
			},
		},
	})
}

func TestOfflineStoreConnectionDataSource_BigQuery(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetOfflineStoreConnection().Return(&serverv1.GetOfflineStoreConnectionResponse{
		Connection: &serverv1.OfflineStoreConnection{
			Id:            "test-conn-id",
			Name:          "bq-conn",
			EnvironmentId: "test-env-id",
			Config: &serverv1.OfflineStoreConnectionConfigStored{
				Config: &serverv1.OfflineStoreConnectionConfigStored_Bigquery{
					Bigquery: &serverv1.BigQueryOfflineStoreConnectionConfig{
						ProjectId: "my-bq-project",
						DatasetId: "my-dataset",
					},
				},
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testOfflineStoreConnectionDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "kind", "bigquery"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "bigquery.project_id", "my-bq-project"),
					resource.TestCheckResourceAttr("data.chalk_offline_store_connection.test", "bigquery.dataset_id", "my-dataset"),
				),
			},
		},
	})
}

// TestOfflineStoreConnectionDataSource_NoPasswordExposure asserts that the
// snowflake nested object does not surface password/private_key fields.
func TestOfflineStoreConnectionDataSource_NoPasswordExposure(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	warehouse := "MY_WH"
	server.OnGetOfflineStoreConnection().Return(&serverv1.GetOfflineStoreConnectionResponse{
		Connection: &serverv1.OfflineStoreConnection{
			Id:            "test-conn-id",
			Name:          "snow-conn",
			EnvironmentId: "test-env-id",
			Config: &serverv1.OfflineStoreConnectionConfigStored{
				Config: &serverv1.OfflineStoreConnectionConfigStored_Snowflake{
					Snowflake: &serverv1.SnowflakeOfflineStoreConnectionConfigStored{
						Credentials: &serverv1.SnowflakeCredentialsStored{
							Account:   "acct",
							Username:  "user",
							Warehouse: &warehouse,
						},
					},
				},
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testOfflineStoreConnectionDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_offline_store_connection.test", "snowflake.password"),
					resource.TestCheckNoResourceAttr("data.chalk_offline_store_connection.test", "snowflake.private_key"),
				),
			},
		},
	})
}

func TestOfflineStoreConnectionDataSource_RpcError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetOfflineStoreConnection().ReturnError(connect.NewError(connect.CodeInternal, errors.New("backend exploded")))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testOfflineStoreConnectionDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read offline store connection`),
			},
		},
	})
}
