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

const testGcpCloudCredentialsDataSourceConfig = `
data "chalk_gcp_cloud_credentials" "test" {
  id = "test-cred-id"
}
`

func buildGcpCredsResponse(sa *string) *serverv1.GetCloudCredentialsResponse {
	return &serverv1.GetCloudCredentialsResponse{
		Credentials: &serverv1.CloudCredentialsResponse{
			Id:   "test-cred-id",
			Name: "test-gcp-creds",
			Kind: "gcp",
			Spec: &serverv1.CloudConfig{
				Config: &serverv1.CloudConfig_Gcp{
					Gcp: &serverv1.GCPCloudConfig{
						ProjectId:                "my-gcp-project",
						Region:                   "us-central1",
						ManagementServiceAccount: sa,
					},
				},
			},
		},
	}
}

func TestGcpCloudCredentialsDataSource_AllAttributes(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	sa := "chalk@my-gcp-project.iam.gserviceaccount.com"
	server.OnGetCloudCredentials().Return(buildGcpCredsResponse(&sa))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testGcpCloudCredentialsDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_gcp_cloud_credentials.test", "id", "test-cred-id"),
					resource.TestCheckResourceAttr("data.chalk_gcp_cloud_credentials.test", "name", "test-gcp-creds"),
					resource.TestCheckResourceAttr("data.chalk_gcp_cloud_credentials.test", "gcp_project_id", "my-gcp-project"),
					resource.TestCheckResourceAttr("data.chalk_gcp_cloud_credentials.test", "gcp_region", "us-central1"),
					resource.TestCheckResourceAttr("data.chalk_gcp_cloud_credentials.test", "gcp_management_service_account", "chalk@my-gcp-project.iam.gserviceaccount.com"),
				),
			},
		},
	})
}

func TestGcpCloudCredentialsDataSource_NoManagementSA(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudCredentials().Return(buildGcpCredsResponse(nil))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testGcpCloudCredentialsDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_gcp_cloud_credentials.test", "gcp_management_service_account"),
				),
			},
		},
	})
}

func TestGcpCloudCredentialsDataSource_RpcError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudCredentials().ReturnError(connect.NewError(connect.CodeInternal, errors.New("backend exploded")))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testGcpCloudCredentialsDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read GCP cloud credentials`),
			},
		},
	})
}
