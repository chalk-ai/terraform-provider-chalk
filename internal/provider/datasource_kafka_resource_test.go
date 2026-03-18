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

func TestDatasourceKafkaResourceCreate(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server = { literal = "kafka1.example.com:9092,kafka2.example.com:9092" }
  topic            = { literal = "my-topic" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "id", "test-integration-id"),
					resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "name", "my-kafka"),
					resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "environment_id", "test-env-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1, "Expected exactly one InsertIntegration call")

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "my-kafka", req.Name)
						assert.Equal(t, serverv1.IntegrationKind_INTEGRATION_KIND_KAFKA, req.IntegrationKind)
						assert.Equal(t, "kafka1.example.com:9092,kafka2.example.com:9092", literalVal(req.Config, "KAFKA_BOOTSTRAP_SERVER"))
						assert.Equal(t, "my-topic", literalVal(req.Config, "KAFKA_TOPIC"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKafkaResourceCreateWithSASL(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server  = { literal = "kafka1.example.com:9092" }
  topic             = { literal = "my-topic" }
  security_protocol = { literal = "SASL_SSL" }
  sasl_mechanism    = { literal = "SCRAM-SHA-256" }
  sasl_username     = { literal = "kafka-user" }
  sasl_password     = { secret_id = "kafka-password-secret" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "SASL_SSL", literalVal(req.Config, "KAFKA_SECURITY_PROTOCOL"))
						assert.Equal(t, "SCRAM-SHA-256", literalVal(req.Config, "KAFKA_SASL_MECHANISM"))
						assert.Equal(t, "kafka-user", literalVal(req.Config, "KAFKA_SASL_USERNAME"))
						assert.Equal(t, "kafka-password-secret", secretIdVal(req.Config, "KAFKA_SASL_PASSWORD"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKafkaResourceCreateWithSSL(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server  = { literal = "kafka1.example.com:9092" }
  topic             = { literal = "my-topic" }
  security_protocol = { literal = "SSL" }
  ssl_ca_file       = { literal = "/path/to/ca.pem" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "SSL", literalVal(req.Config, "KAFKA_SECURITY_PROTOCOL"))
						assert.Equal(t, "/path/to/ca.pem", literalVal(req.Config, "KAFKA_SSL_CA_FILE"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKafkaResourceCreateWithMSKIAM(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server = { literal = "kafka1.example.com:9092" }
  topic            = { literal = "my-topic" }
  msk_iam_auth     = { literal = "true" }
  aws_region       = { literal = "us-east-1" }
  aws_role_arn     = { literal = "arn:aws:iam::123456789:role/KafkaConsumer" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "true", literalVal(req.Config, "KAFKA_MSK_IAM_AUTH"))
						assert.Equal(t, "us-east-1", literalVal(req.Config, "KAFKA_AWS_REGION"))
						assert.Equal(t, "arn:aws:iam::123456789:role/KafkaConsumer", literalVal(req.Config, "KAFKA_AWS_ROLE_ARN"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKafkaResourceCreateWithDLQ(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server = { literal = "kafka1.example.com:9092" }
  topic            = { literal = "my-topic" }
  dlq_topic        = { literal = "my-topic-dlq" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "my-topic-dlq", literalVal(req.Config, "KAFKA_DLQ_TOPIC"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKafkaResourceCreateWithPrefixes(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server = { literal = "kafka1.example.com:9092" }
  topic            = { literal = "my-topic" }
  client_id_prefix = { literal = "chalk-client" }
  group_id_prefix  = { literal = "chalk-group" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "chalk-client", literalVal(req.Config, "KAFKA_CLIENT_ID_PREFIX"))
						assert.Equal(t, "chalk-group", literalVal(req.Config, "KAFKA_GROUP_ID_PREFIX"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKafkaResourceCreateWithAdditionalArgs(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server      = { literal = "kafka1.example.com:9092" }
  topic                 = { literal = "my-topic" }
  additional_kafka_args = { literal = "{\"auto.offset.reset\": \"earliest\"}" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "{\"auto.offset.reset\": \"earliest\"}", literalVal(req.Config, "KAFKA_ADDITIONAL_KAFKA_ARGS"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKafkaResourceUpdate(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server = { literal = "kafka1.example.com:9092" }
  topic            = { literal = "my-topic" }
}
`,
				Check: resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "name", "my-kafka"),
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka-updated"
  environment_id = "test-env-id"

  bootstrap_server  = { literal = "kafka1.example.com:9092,kafka2.example.com:9092" }
  topic             = { literal = "my-topic-new" }
  security_protocol = { literal = "SASL_SSL" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kafka.test", "name", "my-kafka-updated"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateIntegration")
						require.NotEmpty(t, captured)

						req := captured[len(captured)-1].(*serverv1.UpdateIntegrationRequest)
						assert.Equal(t, "my-kafka-updated", req.Name)
						assert.Equal(t, "test-integration-id", req.IntegrationId)
						assert.Equal(t, "kafka1.example.com:9092,kafka2.example.com:9092", literalVal(req.Config, "KAFKA_BOOTSTRAP_SERVER"))
						assert.Equal(t, "my-topic-new", literalVal(req.Config, "KAFKA_TOPIC"))
						assert.Equal(t, "SASL_SSL", literalVal(req.Config, "KAFKA_SECURITY_PROTOCOL"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKafkaResourceDelete(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server = { literal = "kafka1.example.com:9092" }
  topic            = { literal = "my-topic" }
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

func TestDatasourceKafkaResourceImport(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server = { literal = "kafka1.example.com:9092" }
  topic            = { literal = "my-topic" }
}
`,
			},
			{
				ResourceName:      "chalk_datasource_kafka.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "test-env-id/test-integration-id",
			},
		},
	})
}

func TestDatasourceKafkaResourceImportWrongKind(t *testing.T) {
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
				Kind:          serverv1.IntegrationKind_INTEGRATION_KIND_KAFKA,
				EnvironmentId: "test-env-id",
			},
		}, nil
	})

	kinesisName := "my-kinesis"
	server.OnGetIntegration().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.GetIntegrationResponse{
			IntegrationWithSecrets: &serverv1.IntegrationWithSecrets{
				Integration: &serverv1.Integration{
					Id:            "test-integration-id",
					Name:          &kinesisName,
					Kind:          serverv1.IntegrationKind_INTEGRATION_KIND_KINESIS,
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
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server = { literal = "kafka1.example.com:9092" }
  topic            = { literal = "my-topic" }
}
`,
				ExpectError: regexp.MustCompile("Unexpected Integration Kind"),
			},
		},
	})
}

func TestDatasourceKafkaResourceConfigConflict(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kafka" "test" {
  name           = "my-kafka"
  environment_id = "test-env-id"

  bootstrap_server = { literal = "kafka1.example.com:9092" }
  topic            = { literal = "my-topic" }

  config = {
    KAFKA_TOPIC = { literal = "other-topic" }
  }
}
`,
				ExpectError: regexp.MustCompile("Reserved Config Key"),
			},
		},
	})
}
