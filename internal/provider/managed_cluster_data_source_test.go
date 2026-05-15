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

const testManagedClusterDataSourceConfig = `
data "chalk_managed_cluster" "test" {
  id = "test-cluster-id"
}
`

//go:fix inline
func strPtr(s string) *string { return new(s) }

// TestManagedClusterDataSource_AllAttributes verifies the data source surfaces
// every attribute the resource declares.
func TestManagedClusterDataSource_AllAttributes(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudComponentCluster().Return(&serverv1.GetCloudComponentClusterResponse{
		Cluster: &serverv1.CloudComponentClusterResponse{
			Id:                "test-cluster-id",
			Kind:              "EKS_STANDARD",
			Designator:        new("abcd"),
			CloudCredentialId: new("test-cred-id"),
			VpcId:             new("test-vpc-id"),
			Spec: &serverv1.CloudComponentCluster{
				Name: "test-cluster-name",
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testManagedClusterDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_managed_cluster.test", "id", "test-cluster-id"),
					resource.TestCheckResourceAttr("data.chalk_managed_cluster.test", "name", "test-cluster-name"),
					resource.TestCheckResourceAttr("data.chalk_managed_cluster.test", "kind", "EKS_STANDARD"),
					resource.TestCheckResourceAttr("data.chalk_managed_cluster.test", "designator", "abcd"),
					resource.TestCheckResourceAttr("data.chalk_managed_cluster.test", "cloud_credential_id", "test-cred-id"),
					resource.TestCheckResourceAttr("data.chalk_managed_cluster.test", "vpc_id", "test-vpc-id"),
				),
			},
		},
	})
}

// TestManagedClusterDataSource_NullDesignator verifies that a missing designator
// (optional field on the proto) maps to a null string in state, not the empty string.
func TestManagedClusterDataSource_NullDesignator(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudComponentCluster().Return(&serverv1.GetCloudComponentClusterResponse{
		Cluster: &serverv1.CloudComponentClusterResponse{
			Id:                "test-cluster-id",
			Kind:              "EKS_STANDARD",
			Designator:        nil,
			CloudCredentialId: new("test-cred-id"),
			VpcId:             new("test-vpc-id"),
			Spec:              &serverv1.CloudComponentCluster{Name: "test-cluster-name"},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testManagedClusterDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_managed_cluster.test", "designator"),
				),
			},
		},
	})
}

// TestManagedClusterDataSource_RpcError verifies that gRPC errors surface as
// Terraform diagnostic errors rather than silent state.
func TestManagedClusterDataSource_RpcError(t *testing.T) {
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
				Config:      providerConfig(server.URL) + testManagedClusterDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read managed cluster`),
			},
		},
	})
}
