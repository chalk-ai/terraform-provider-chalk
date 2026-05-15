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

const testKubernetesClusterDataSourceConfig = `
data "chalk_kubernetes_cluster" "test" {
  id = "test-cluster-id"
}
`

func buildKubeClusterResponse(credID *string, dnsZone *string) *serverv1.GetCloudComponentClusterResponse {
	return &serverv1.GetCloudComponentClusterResponse{
		Cluster: &serverv1.CloudComponentClusterResponse{
			Id:                "test-cluster-id",
			Kind:              "EKS_STANDARD",
			TeamId:            "team-abc",
			CloudCredentialId: credID,
			Spec: &serverv1.CloudComponentCluster{
				Name:    "test-cluster-name",
				DnsZone: dnsZone,
			},
		},
	}
}

func TestKubernetesClusterDataSource_AllAttributes(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	credID := "test-cred-id"
	dns := "test.chalk.ai"
	server.OnGetCloudComponentCluster().Return(buildKubeClusterResponse(&credID, &dns))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testKubernetesClusterDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_kubernetes_cluster.test", "id", "test-cluster-id"),
					resource.TestCheckResourceAttr("data.chalk_kubernetes_cluster.test", "name", "test-cluster-name"),
					resource.TestCheckResourceAttr("data.chalk_kubernetes_cluster.test", "kind", "EKS_STANDARD"),
					resource.TestCheckResourceAttr("data.chalk_kubernetes_cluster.test", "cloud_credential_id", "test-cred-id"),
					resource.TestCheckResourceAttr("data.chalk_kubernetes_cluster.test", "dns_zone", "test.chalk.ai"),
					resource.TestCheckResourceAttr("data.chalk_kubernetes_cluster.test", "team_id", "team-abc"),
				),
			},
		},
	})
}

func TestKubernetesClusterDataSource_NoCloudCredential(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudComponentCluster().Return(buildKubeClusterResponse(nil, nil))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testKubernetesClusterDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_kubernetes_cluster.test", "cloud_credential_id"),
					resource.TestCheckNoResourceAttr("data.chalk_kubernetes_cluster.test", "dns_zone"),
				),
			},
		},
	})
}

func TestKubernetesClusterDataSource_RpcError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudComponentCluster().ReturnError(
		connect.NewError(connect.CodeInternal, errors.New("backend exploded")),
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testKubernetesClusterDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read kubernetes cluster`),
			},
		},
	})
}
