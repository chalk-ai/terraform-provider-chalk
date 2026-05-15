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

const testUnmanagedBGPDataSourceConfig = `
data "chalk_unmanaged_cluster_background_persistence" "test" {
  id = "test-bgp-id"
}
`

func buildKafkaBGPResponse() *serverv1.GetClusterBackgroundPersistenceResponse {
	clusterID := "test-cluster-id"
	return &serverv1.GetClusterBackgroundPersistenceResponse{
		BackgroundPersistence: &serverv1.BackgroundPersistence{
			Id:            "test-bgp-id",
			KubeClusterId: &clusterID,
			Specs: &serverv1.BackgroundPersistenceDeploymentSpecs{
				CommonPersistenceSpecs: &serverv1.BackgroundPersistenceCommonSpecs{
					ServiceAccountName:          "test-sa",
					Namespace:                   "background-persistence",
					BqUploadBucket:              "s3://test-bucket",
					KafkaDlqTopic:               "dlq",
					BqUploadTopic:               "offline-bulk-insert",
					BigqueryStreamingWriteTopic: "offline-streaming-insert",
					MetricsBusTopicId:           "metrics-bus",
					ResultBusTopicId:            "result-bus",
				},
				ApiServerHost:         "https://api.example.com",
				KafkaBootstrapServers: "broker1:9092",
				KafkaSaslMechanism:    "AWS_MSK_IAM",
				KafkaSecurityProtocol: "SASL_SSL",
			},
		},
	}
}

func TestUnmanagedClusterBackgroundPersistenceDataSource_Kafka(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetClusterBackgroundPersistence().Return(buildKafkaBGPResponse())

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testUnmanagedBGPDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "id", "test-bgp-id"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "kube_cluster_id", "test-cluster-id"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "service_account_name", "test-sa"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "namespace", "background-persistence"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "offline_store_upload_bucket_name", "s3://test-bucket"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "api_server_host", "https://api.example.com"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "kafka.bootstrap_servers", "broker1:9092"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "kafka.dlq_topic", "dlq"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "kafka.metrics_bus_topic_id", "metrics-bus"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "kafka.result_bus_topic_id", "result-bus"),
				),
			},
		},
	})
}

// TestUnmanagedClusterBackgroundPersistenceDataSource_NoAutodiscoverKey asserts
// that the schema deliberately omits autodiscover_key (Sensitive on the
// resource).
func TestUnmanagedClusterBackgroundPersistenceDataSource_NoAutodiscoverKey(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	r := buildKafkaBGPResponse()
	r.BackgroundPersistence.Specs.AutodiscoverKey = stringPointer("super-secret-key")
	server.OnGetClusterBackgroundPersistence().Return(r)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testUnmanagedBGPDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_unmanaged_cluster_background_persistence.test", "autodiscover_key"),
				),
			},
		},
	})
}

func TestUnmanagedClusterBackgroundPersistenceDataSource_RpcError(t *testing.T) {
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
				Config:      providerConfig(server.URL) + testUnmanagedBGPDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read unmanaged cluster background persistence`),
			},
		},
	})
}

func stringPointer(s string) *string { return &s }
