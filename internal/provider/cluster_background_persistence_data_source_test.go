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

const testLegacyBGPDataSourceConfig = `
data "chalk_cluster_background_persistence" "test" {
  id = "test-legacy-bgp-id"
}
`

func TestLegacyClusterBackgroundPersistenceDataSource_AllAttributes(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	clusterID := "test-cluster-id"
	server.OnGetClusterBackgroundPersistence().Return(&serverv1.GetClusterBackgroundPersistenceResponse{
		BackgroundPersistence: &serverv1.BackgroundPersistence{
			Id:            "test-legacy-bgp-id",
			KubeClusterId: &clusterID,
			Specs: &serverv1.BackgroundPersistenceDeploymentSpecs{
				CommonPersistenceSpecs: &serverv1.BackgroundPersistenceCommonSpecs{
					Namespace:          "bgp-ns",
					ServiceAccountName: "bgp-sa",
					GoogleCloudProject: "my-gcp-project",
					KafkaDlqTopic:      "dlq",
					MetricsBusTopicId:  "metrics",
					ResultBusTopicId:   "result",
				},
				ApiServerHost:                   "https://api.example.com",
				KafkaBootstrapServers:           "broker1:9092",
				SnowflakeStorageIntegrationName: "snow_int",
				RedisLightningSupportsHasMany:   true,
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testLegacyBGPDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "id", "test-legacy-bgp-id"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "kube_cluster_id", "test-cluster-id"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "namespace", "bgp-ns"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "service_account_name", "bgp-sa"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "google_cloud_project", "my-gcp-project"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "kafka_dlq_topic", "dlq"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "metrics_bus_topic_id", "metrics"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "result_bus_topic_id", "result"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "api_server_host", "https://api.example.com"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "kafka_bootstrap_servers", "broker1:9092"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "snowflake_storage_integration_name", "snow_int"),
					resource.TestCheckResourceAttr("data.chalk_cluster_background_persistence.test", "redis_lightning_supports_has_many", "true"),
				),
			},
		},
	})
}

// TestLegacyClusterBackgroundPersistenceDataSource_NoKafkaSaslSecret asserts the
// schema does not surface kafka_sasl_secret (Sensitive on the resource).
func TestLegacyClusterBackgroundPersistenceDataSource_NoKafkaSaslSecret(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	clusterID := "test-cluster-id"
	server.OnGetClusterBackgroundPersistence().Return(&serverv1.GetClusterBackgroundPersistenceResponse{
		BackgroundPersistence: &serverv1.BackgroundPersistence{
			Id:            "test-legacy-bgp-id",
			KubeClusterId: &clusterID,
			Specs: &serverv1.BackgroundPersistenceDeploymentSpecs{
				CommonPersistenceSpecs: &serverv1.BackgroundPersistenceCommonSpecs{
					Namespace:          "bgp-ns",
					ServiceAccountName: "bgp-sa",
				},
				KafkaSaslSecret: "should-not-appear",
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testLegacyBGPDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_cluster_background_persistence.test", "kafka_sasl_secret"),
				),
			},
		},
	})
}

func TestLegacyClusterBackgroundPersistenceDataSource_RpcError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetClusterBackgroundPersistence().ReturnError(
		connect.NewError(connect.CodeInternal, errors.New("backend exploded")),
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testLegacyBGPDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read cluster background persistence`),
			},
		},
	})
}
