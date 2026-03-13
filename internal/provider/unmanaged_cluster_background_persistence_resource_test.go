package provider

import (
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

const testBGPWritersHCL = `
  writers = [
    {
      bus_subscriber_type   = "GO_METRICS_BUS_WRITER"
      default_replica_count = 1
      request = {
        cpu    = "500m"
        memory = "1Gi"
      }
    },
    {
      bus_subscriber_type   = "CLUSTER_MANAGER"
      default_replica_count = 1
      request = {
        cpu    = "500m"
        memory = "1Gi"
      }
    }
  ]
`

func setupMockBuilderServerBGP(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	var currentBGP *serverv1.BackgroundPersistence

	server.OnCreateClusterBackgroundPersistence().WithBehavior(func(req proto.Message) (proto.Message, error) {
		createReq := req.(*serverv1.CreateClusterBackgroundPersistenceRequest)
		kubeClusterID := createReq.GetKubeClusterId()
		currentBGP = &serverv1.BackgroundPersistence{
			Id:            "test-bgp-id",
			Specs:         createReq.Specs,
			KubeClusterId: &kubeClusterID,
		}
		return &serverv1.CreateClusterBackgroundPersistenceResponse{Id: "test-bgp-id"}, nil
	})

	server.OnGetClusterBackgroundPersistence().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if currentBGP == nil {
			return nil, connect.NewError(connect.CodeNotFound, nil)
		}
		return &serverv1.GetClusterBackgroundPersistenceResponse{BackgroundPersistence: currentBGP}, nil
	})

	return server
}

func TestUnmanagedClusterBGPCreateWithGooglePubSub(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerBGP(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cluster_background_persistence" "test" {
  kube_cluster_id     = "test-kube-cluster"
  service_account_name = "test-sa"
  namespace           = "default"
` + testBGPWritersHCL + `
  google_pubsub = {
    offline_store_upload_bus = {
      subscription_id = "upload-sub"
      topic_id        = "upload-topic"
    }
    offline_store_streaming_write_bus = {
      subscription_id = "streaming-sub"
      topic_id        = "streaming-topic"
    }
    metrics_bus = {
      subscription_id = "metrics-sub"
      topic_id        = "metrics-topic"
    }
    result_bus = {
      offline_store_subscription_id = "result-offline-sub"
      online_store_subscription_id  = "result-online-sub"
      topic_id                      = "result-topic"
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "id", "test-bgp-id"),
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "google_pubsub.offline_store_upload_bus.subscription_id", "upload-sub"),
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "google_pubsub.offline_store_upload_bus.topic_id", "upload-topic"),
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "google_pubsub.metrics_bus.subscription_id", "metrics-sub"),
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "google_pubsub.result_bus.topic_id", "result-topic"),
					resource.TestCheckNoResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "kafka"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateClusterBackgroundPersistence")
						require.Len(t, captured, 1, "Expected exactly one CreateClusterBackgroundPersistence call")

						req := captured[0].(*serverv1.CreateClusterBackgroundPersistenceRequest)
						assert.Equal(t, "test-kube-cluster", req.GetKubeClusterId())
						assert.Equal(t, "upload-sub", req.Specs.CommonPersistenceSpecs.BigqueryParquetUploadSubscriptionId)
						assert.Equal(t, "upload-topic", req.Specs.CommonPersistenceSpecs.BqUploadTopic)
						assert.Equal(t, "metrics-sub", req.Specs.CommonPersistenceSpecs.MetricsBusSubscriptionId)
						assert.Equal(t, "metrics-topic", req.Specs.CommonPersistenceSpecs.MetricsBusTopicId)
						assert.Equal(t, "result-offline-sub", req.Specs.CommonPersistenceSpecs.ResultBusOfflineStoreSubscriptionId)
						assert.Equal(t, "result-online-sub", req.Specs.CommonPersistenceSpecs.ResultBusOnlineStoreSubscriptionId)
						assert.Equal(t, "result-topic", req.Specs.CommonPersistenceSpecs.ResultBusTopicId)
						require.Len(t, req.Specs.Writers, 2)
						assert.Equal(t, "GO_METRICS_BUS_WRITER", req.Specs.Writers[0].BusSubscriberType)
						assert.Equal(t, "CLUSTER_MANAGER", req.Specs.Writers[1].BusSubscriberType)
						return nil
					},
				),
			},
		},
	})
}

func TestUnmanagedClusterBGPCreateWithKafka(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerBGP(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cluster_background_persistence" "test" {
  kube_cluster_id     = "test-kube-cluster"
  service_account_name = "test-sa"
  namespace           = "default"
` + testBGPWritersHCL + `
  kafka = {
    sasl_secret       = "my-sasl-secret"
    bootstrap_servers = "kafka:9092"
    dlq_topic         = "my-dlq-topic"
    offline_store_bus_upload_topic_id          = "upload-topic"
    offline_store_bus_streaming_write_topic_id = "streaming-topic"
    metrics_bus_topic_id = "metrics-topic"
    result_bus_topic_id  = "result-topic"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "id", "test-bgp-id"),
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "kafka.sasl_secret", "my-sasl-secret"),
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "kafka.bootstrap_servers", "kafka:9092"),
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "kafka.dlq_topic", "my-dlq-topic"),
					resource.TestCheckNoResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "google_pubsub"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateClusterBackgroundPersistence")
						require.Len(t, captured, 1, "Expected exactly one CreateClusterBackgroundPersistence call")

						req := captured[0].(*serverv1.CreateClusterBackgroundPersistenceRequest)
						assert.Equal(t, "my-sasl-secret", req.Specs.KafkaSaslSecret)
						assert.Equal(t, "kafka:9092", req.Specs.KafkaBootstrapServers)
						assert.Equal(t, "my-dlq-topic", req.Specs.CommonPersistenceSpecs.KafkaDlqTopic)
						return nil
					},
				),
			},
		},
	})
}

func TestUnmanagedClusterBGPUpdate(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerBGP(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cluster_background_persistence" "test" {
  kube_cluster_id     = "test-kube-cluster"
  service_account_name = "test-sa"
  namespace           = "default"
` + testBGPWritersHCL + `
  kafka = {
    sasl_secret       = "my-sasl-secret"
    bootstrap_servers = "kafka:9092"
    dlq_topic         = "original-dlq-topic"
    offline_store_bus_upload_topic_id          = "upload-topic"
    offline_store_bus_streaming_write_topic_id = "streaming-topic"
    metrics_bus_topic_id = "metrics-topic"
    result_bus_topic_id  = "result-topic"
  }
}
`,
				Check: resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "kafka.dlq_topic", "original-dlq-topic"),
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cluster_background_persistence" "test" {
  kube_cluster_id     = "test-kube-cluster"
  service_account_name = "test-sa"
  namespace           = "default"
` + testBGPWritersHCL + `
  kafka = {
    sasl_secret       = "my-sasl-secret"
    bootstrap_servers = "kafka:9092"
    dlq_topic         = "updated-dlq-topic"
    offline_store_bus_upload_topic_id          = "upload-topic"
    offline_store_bus_streaming_write_topic_id = "streaming-topic"
    metrics_bus_topic_id = "metrics-topic"
    result_bus_topic_id  = "result-topic"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "kafka.dlq_topic", "updated-dlq-topic"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateClusterBackgroundPersistence")
						require.Len(t, captured, 2, "Expected two CreateClusterBackgroundPersistence calls (create + update upsert)")

						req := captured[1].(*serverv1.CreateClusterBackgroundPersistenceRequest)
						assert.Equal(t, "updated-dlq-topic", req.Specs.CommonPersistenceSpecs.KafkaDlqTopic)
						return nil
					},
				),
			},
		},
	})
}

func TestUnmanagedClusterBGPClearWriters(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerBGP(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cluster_background_persistence" "test" {
  kube_cluster_id      = "test-kube-cluster"
  service_account_name = "test-sa"
  namespace            = "default"
` + testBGPWritersHCL + `
  kafka = {
    sasl_secret       = "my-sasl-secret"
    bootstrap_servers = "kafka:9092"
    dlq_topic         = "my-dlq-topic"
    offline_store_bus_upload_topic_id          = "upload-topic"
    offline_store_bus_streaming_write_topic_id = "streaming-topic"
    metrics_bus_topic_id = "metrics-topic"
    result_bus_topic_id  = "result-topic"
  }
}
`,
				Check: resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "writers.#", "2"),
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cluster_background_persistence" "test" {
  kube_cluster_id      = "test-kube-cluster"
  service_account_name = "test-sa"
  namespace            = "default"
  writers              = []
  kafka = {
    sasl_secret       = "my-sasl-secret"
    bootstrap_servers = "kafka:9092"
    dlq_topic         = "my-dlq-topic"
    offline_store_bus_upload_topic_id          = "upload-topic"
    offline_store_bus_streaming_write_topic_id = "streaming-topic"
    metrics_bus_topic_id = "metrics-topic"
    result_bus_topic_id  = "result-topic"
  }
}
`,
				Check: resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "writers.#", "0"),
			},
		},
	})
}

func TestUnmanagedClusterBGPReadNotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	var currentBGP *serverv1.BackgroundPersistence
	var returnNil bool

	server.OnCreateClusterBackgroundPersistence().WithBehavior(func(req proto.Message) (proto.Message, error) {
		createReq := req.(*serverv1.CreateClusterBackgroundPersistenceRequest)
		kubeClusterID := createReq.GetKubeClusterId()
		currentBGP = &serverv1.BackgroundPersistence{
			Id:            "test-bgp-id",
			Specs:         createReq.Specs,
			KubeClusterId: &kubeClusterID,
		}
		return &serverv1.CreateClusterBackgroundPersistenceResponse{Id: "test-bgp-id"}, nil
	})

	server.OnGetClusterBackgroundPersistence().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if returnNil {
			return &serverv1.GetClusterBackgroundPersistenceResponse{}, nil
		}
		return &serverv1.GetClusterBackgroundPersistenceResponse{BackgroundPersistence: currentBGP}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cluster_background_persistence" "test" {
  kube_cluster_id      = "test-kube-cluster"
  service_account_name = "test-sa"
  namespace            = "default"
` + testBGPWritersHCL + `
  kafka = {
    sasl_secret       = "my-sasl-secret"
    bootstrap_servers = "kafka:9092"
    dlq_topic         = "my-dlq-topic"
    offline_store_bus_upload_topic_id          = "upload-topic"
    offline_store_bus_streaming_write_topic_id = "streaming-topic"
    metrics_bus_topic_id = "metrics-topic"
    result_bus_topic_id  = "result-topic"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "id", "test-bgp-id"),
					// Flip to nil-mode before the post-step plan check runs, so Read removes
					// the resource from state and we get a non-empty plan.
					func(s *terraform.State) error {
						returnNil = true
						return nil
					},
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestUnmanagedClusterBGPApiServerHostDefaultsToProvider(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerBGP(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cluster_background_persistence" "test" {
  kube_cluster_id      = "test-kube-cluster"
  service_account_name = "test-sa"
  namespace            = "default"
` + testBGPWritersHCL + `
  kafka = {
    sasl_secret       = "my-sasl-secret"
    bootstrap_servers = "kafka:9092"
    dlq_topic         = "my-dlq-topic"
    offline_store_bus_upload_topic_id          = "upload-topic"
    offline_store_bus_streaming_write_topic_id = "streaming-topic"
    metrics_bus_topic_id = "metrics-topic"
    result_bus_topic_id  = "result-topic"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "api_server_host", server.URL),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateClusterBackgroundPersistence")
						require.Len(t, captured, 1, "Expected exactly one CreateClusterBackgroundPersistence call")

						req := captured[0].(*serverv1.CreateClusterBackgroundPersistenceRequest)
						assert.Equal(t, server.URL, req.Specs.ApiServerHost)
						return nil
					},
				),
			},
		},
	})
}

func TestUnmanagedClusterBGPApiServerHostExplicit(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerBGP(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cluster_background_persistence" "test" {
  kube_cluster_id      = "test-kube-cluster"
  service_account_name = "test-sa"
  namespace            = "default"
  api_server_host      = "https://custom-api-server.example.com"
` + testBGPWritersHCL + `
  kafka = {
    sasl_secret       = "my-sasl-secret"
    bootstrap_servers = "kafka:9092"
    dlq_topic         = "my-dlq-topic"
    offline_store_bus_upload_topic_id          = "upload-topic"
    offline_store_bus_streaming_write_topic_id = "streaming-topic"
    metrics_bus_topic_id = "metrics-topic"
    result_bus_topic_id  = "result-topic"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "api_server_host", "https://custom-api-server.example.com"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateClusterBackgroundPersistence")
						require.Len(t, captured, 1, "Expected exactly one CreateClusterBackgroundPersistence call")

						req := captured[0].(*serverv1.CreateClusterBackgroundPersistenceRequest)
						assert.Equal(t, "https://custom-api-server.example.com", req.Specs.ApiServerHost)
						return nil
					},
				),
			},
		},
	})
}

func TestUnmanagedClusterBGPImport(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerBGP(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cluster_background_persistence" "test" {
  kube_cluster_id     = "test-kube-cluster"
  service_account_name = "test-sa"
  namespace           = "default"
` + testBGPWritersHCL + `
  kafka = {
    sasl_secret       = "my-sasl-secret"
    bootstrap_servers = "kafka:9092"
    dlq_topic         = "my-dlq-topic"
    offline_store_bus_upload_topic_id          = "upload-topic"
    offline_store_bus_streaming_write_topic_id = "streaming-topic"
    metrics_bus_topic_id = "metrics-topic"
    result_bus_topic_id  = "result-topic"
  }
}
`,
				Check: resource.TestCheckResourceAttr("chalk_unmanaged_cluster_background_persistence.test", "id", "test-bgp-id"),
			},
			{
				ResourceName:      "chalk_unmanaged_cluster_background_persistence.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
