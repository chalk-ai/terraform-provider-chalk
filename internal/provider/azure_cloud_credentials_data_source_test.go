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

const testAzureCloudCredentialsDataSourceConfig = `
data "chalk_azure_cloud_credentials" "test" {
  id = "test-cred-id"
}
`

func buildAzureCredsResponse() *serverv1.GetCloudCredentialsResponse {
	return &serverv1.GetCloudCredentialsResponse{
		Credentials: &serverv1.CloudCredentialsResponse{
			Id:   "test-cred-id",
			Name: "test-azure-creds",
			Kind: "azure",
			Spec: &serverv1.CloudConfig{
				Config: &serverv1.CloudConfig_Azure{
					Azure: &serverv1.AzureCloudConfig{
						SubscriptionId: "11111111-1111-1111-1111-111111111111",
						TenantId:       "22222222-2222-2222-2222-222222222222",
						Region:         "eastus",
						ResourceGroup:  "chalk-rg",
					},
				},
			},
		},
	}
}

func TestAzureCloudCredentialsDataSource_AllAttributes(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudCredentials().Return(buildAzureCredsResponse())

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testAzureCloudCredentialsDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_azure_cloud_credentials.test", "id", "test-cred-id"),
					resource.TestCheckResourceAttr("data.chalk_azure_cloud_credentials.test", "name", "test-azure-creds"),
					resource.TestCheckResourceAttr("data.chalk_azure_cloud_credentials.test", "subscription_id", "11111111-1111-1111-1111-111111111111"),
					resource.TestCheckResourceAttr("data.chalk_azure_cloud_credentials.test", "tenant_id", "22222222-2222-2222-2222-222222222222"),
					resource.TestCheckResourceAttr("data.chalk_azure_cloud_credentials.test", "region", "eastus"),
					resource.TestCheckResourceAttr("data.chalk_azure_cloud_credentials.test", "resource_group", "chalk-rg"),
				),
			},
		},
	})
}

func TestAzureCloudCredentialsDataSource_RpcError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudCredentials().ReturnError(connect.NewError(connect.CodeInternal, errors.New("backend exploded")))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testAzureCloudCredentialsDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read Azure cloud credentials`),
			},
		},
	})
}
