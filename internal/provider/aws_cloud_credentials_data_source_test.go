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

const testAwsCloudCredentialsDataSourceConfig = `
data "chalk_aws_cloud_credentials" "test" {
  id = "test-cred-id"
}
`

func buildAwsCredsResponse(externalID *string) *serverv1.GetCloudCredentialsResponse {
	return &serverv1.GetCloudCredentialsResponse{
		Credentials: &serverv1.CloudCredentialsResponse{
			Id:   "test-cred-id",
			Name: "test-creds",
			Kind: "aws",
			Spec: &serverv1.CloudConfig{
				Config: &serverv1.CloudConfig_Aws{
					Aws: &serverv1.AWSCloudConfig{
						AccountId:         "123456789012",
						ManagementRoleArn: "arn:aws:iam::123456789012:role/chalk-mgmt",
						Region:            "us-east-1",
						ExternalId:        externalID,
					},
				},
			},
		},
	}
}

func TestAwsCloudCredentialsDataSource_AllAttributes(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	extID := "ext-12345"
	server.OnGetCloudCredentials().Return(buildAwsCredsResponse(&extID))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testAwsCloudCredentialsDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_aws_cloud_credentials.test", "id", "test-cred-id"),
					resource.TestCheckResourceAttr("data.chalk_aws_cloud_credentials.test", "name", "test-creds"),
					resource.TestCheckResourceAttr("data.chalk_aws_cloud_credentials.test", "aws_account_id", "123456789012"),
					resource.TestCheckResourceAttr("data.chalk_aws_cloud_credentials.test", "aws_management_role_arn", "arn:aws:iam::123456789012:role/chalk-mgmt"),
					resource.TestCheckResourceAttr("data.chalk_aws_cloud_credentials.test", "aws_region", "us-east-1"),
					resource.TestCheckResourceAttr("data.chalk_aws_cloud_credentials.test", "aws_external_id", "ext-12345"),
				),
			},
		},
	})
}

func TestAwsCloudCredentialsDataSource_NoExternalId(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudCredentials().Return(buildAwsCredsResponse(nil))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testAwsCloudCredentialsDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_aws_cloud_credentials.test", "aws_external_id"),
				),
			},
		},
	})
}

func TestAwsCloudCredentialsDataSource_RpcError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudCredentials().ReturnError(
		connect.NewError(connect.CodeInternal, errors.New("backend exploded")),
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testAwsCloudCredentialsDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read AWS cloud credentials`),
			},
		},
	})
}
