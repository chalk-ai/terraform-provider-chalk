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

func TestDatasourceKinesisResourceCreate(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis"
  environment_id = "test-env-id"

  region_name = { literal = "us-east-2" }
  stream_name = { literal = "my-stream" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kinesis.test", "id", "test-integration-id"),
					resource.TestCheckResourceAttr("chalk_datasource_kinesis.test", "name", "my-kinesis"),
					resource.TestCheckResourceAttr("chalk_datasource_kinesis.test", "environment_id", "test-env-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1, "Expected exactly one InsertIntegration call")

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "my-kinesis", req.Name)
						assert.Equal(t, serverv1.IntegrationKind_INTEGRATION_KIND_KINESIS, req.IntegrationKind)
						assert.Equal(t, "us-east-2", literalVal(req.Config, "KINESIS_REGION_NAME"))
						assert.Equal(t, "my-stream", literalVal(req.Config, "KINESIS_STREAM_NAME"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKinesisResourceCreateWithStreamArn(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis"
  environment_id = "test-env-id"

  region_name = { literal = "us-east-2" }
  stream_arn  = { literal = "arn:aws:kinesis:us-east-2:123456789:stream/my-stream" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kinesis.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "arn:aws:kinesis:us-east-2:123456789:stream/my-stream", literalVal(req.Config, "KINESIS_STREAM_ARN"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKinesisResourceCreateWithCredentials(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis"
  environment_id = "test-env-id"

  region_name           = { literal = "us-east-2" }
  stream_name           = { literal = "my-stream" }
  aws_access_key_id     = { literal = "AKIAIOSFODNN7EXAMPLE" }
  aws_secret_access_key = { secret_id = "kinesis-secret-key" }
  aws_session_token     = { literal = "session-token-123" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kinesis.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", literalVal(req.Config, "KINESIS_AWS_ACCESS_KEY_ID"))
						assert.Equal(t, "kinesis-secret-key", secretIdVal(req.Config, "KINESIS_AWS_SECRET_ACCESS_KEY"))
						assert.Equal(t, "session-token-123", literalVal(req.Config, "KINESIS_AWS_SESSION_TOKEN"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKinesisResourceCreateWithDLQ(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis"
  environment_id = "test-env-id"

  region_name                    = { literal = "us-east-2" }
  stream_name                    = { literal = "my-stream" }
  late_arrival_deadline          = { literal = "1hr30m" }
  dead_letter_queue_stream_name  = { literal = "my-stream-dlq" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kinesis.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "1hr30m", literalVal(req.Config, "KINESIS_LATE_ARRIVAL_DEADLINE"))
						assert.Equal(t, "my-stream-dlq", literalVal(req.Config, "KINESIS_DEAD_LETTER_QUEUE_STREAM_NAME"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKinesisResourceCreateWithFanout(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis"
  environment_id = "test-env-id"

  region_name                     = { literal = "us-east-2" }
  stream_name                     = { literal = "my-stream" }
  consumer_role_arn               = { literal = "arn:aws:iam::123456789:role/KinesisConsumer" }
  enhanced_fanout_consumer_name   = { literal = "my-fanout-consumer" }
  endpoint_url                    = { literal = "https://kinesis.us-east-2.amazonaws.com" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kinesis.test", "id", "test-integration-id"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("InsertIntegration")
						require.Len(t, captured, 1)

						req := captured[0].(*serverv1.InsertIntegrationRequest)
						assert.Equal(t, "arn:aws:iam::123456789:role/KinesisConsumer", literalVal(req.Config, "KINESIS_CONSUMER_ROLE_ARN"))
						assert.Equal(t, "my-fanout-consumer", literalVal(req.Config, "KINESIS_ENHANCED_FANOUT_CONSUMER_NAME"))
						assert.Equal(t, "https://kinesis.us-east-2.amazonaws.com", literalVal(req.Config, "KINESIS_ENDPOINT_URL"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKinesisResourceUpdate(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis"
  environment_id = "test-env-id"

  region_name = { literal = "us-east-2" }
  stream_name = { literal = "my-stream" }
}
`,
				Check: resource.TestCheckResourceAttr("chalk_datasource_kinesis.test", "name", "my-kinesis"),
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis-updated"
  environment_id = "test-env-id"

  region_name           = { literal = "us-west-2" }
  stream_name           = { literal = "my-stream-new" }
  late_arrival_deadline = { literal = "30m" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_datasource_kinesis.test", "name", "my-kinesis-updated"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateIntegration")
						require.NotEmpty(t, captured)

						req := captured[len(captured)-1].(*serverv1.UpdateIntegrationRequest)
						assert.Equal(t, "my-kinesis-updated", req.Name)
						assert.Equal(t, "test-integration-id", req.IntegrationId)
						assert.Equal(t, "us-west-2", literalVal(req.Config, "KINESIS_REGION_NAME"))
						assert.Equal(t, "my-stream-new", literalVal(req.Config, "KINESIS_STREAM_NAME"))
						assert.Equal(t, "30m", literalVal(req.Config, "KINESIS_LATE_ARRIVAL_DEADLINE"))

						return nil
					},
				),
			},
		},
	})
}

func TestDatasourceKinesisResourceDelete(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis"
  environment_id = "test-env-id"

  region_name = { literal = "us-east-2" }
  stream_name = { literal = "my-stream" }
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

func TestDatasourceKinesisResourceImport(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis"
  environment_id = "test-env-id"

  region_name = { literal = "us-east-2" }
  stream_name = { literal = "my-stream" }
}
`,
			},
			{
				ResourceName:      "chalk_datasource_kinesis.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "test-env-id/test-integration-id",
			},
		},
	})
}

func TestDatasourceKinesisResourceImportWrongKind(t *testing.T) {
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
				Kind:          serverv1.IntegrationKind_INTEGRATION_KIND_KINESIS,
				EnvironmentId: "test-env-id",
			},
		}, nil
	})

	kafkaName := "my-kafka"
	server.OnGetIntegration().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.GetIntegrationResponse{
			IntegrationWithSecrets: &serverv1.IntegrationWithSecrets{
				Integration: &serverv1.Integration{
					Id:            "test-integration-id",
					Name:          &kafkaName,
					Kind:          serverv1.IntegrationKind_INTEGRATION_KIND_KAFKA,
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
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis"
  environment_id = "test-env-id"

  region_name = { literal = "us-east-2" }
  stream_name = { literal = "my-stream" }
}
`,
				ExpectError: regexp.MustCompile("Unexpected Integration Kind"),
			},
		},
	})
}

func TestDatasourceKinesisResourceConfigConflict(t *testing.T) {
	t.Parallel()
	server := setupMockIntegrationsServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_datasource_kinesis" "test" {
  name           = "my-kinesis"
  environment_id = "test-env-id"

  region_name = { literal = "us-east-2" }
  stream_name = { literal = "my-stream" }

  config = {
    KINESIS_REGION_NAME = { literal = "us-west-1" }
  }
}
`,
				ExpectError: regexp.MustCompile("Reserved Config Key"),
			},
		},
	})
}
