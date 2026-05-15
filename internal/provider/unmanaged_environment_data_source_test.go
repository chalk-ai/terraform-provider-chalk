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

const testUnmanagedEnvironmentDataSourceConfig = `
data "chalk_unmanaged_environment" "test" {
  id = "test-env-id"
}
`

func TestUnmanagedEnvironmentDataSource_AllAttributes(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	clusterID := "test-kube-cluster-id"
	jobNs := "chalk-ns"
	sa := "chalk-sa"
	mode := "BYOC_FULL"

	server.OnGetEnv().Return(&serverv1.GetEnvResponse{
		Environment: &serverv1.Environment{
			Id:                     "test-env-id",
			Name:                   "test-env",
			ProjectId:              "test-project-id",
			Managed:                boolPtr(false),
			KubeClusterId:          &clusterID,
			KubeJobNamespace:       &jobNs,
			KubeServiceAccountName: &sa,
			KubeClusterMode:        &mode,
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testUnmanagedEnvironmentDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_unmanaged_environment.test", "id", "test-env-id"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_environment.test", "name", "test-env"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_environment.test", "project_id", "test-project-id"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_environment.test", "kube_cluster_id", "test-kube-cluster-id"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_environment.test", "kube_job_namespace", "chalk-ns"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_environment.test", "kube_service_account_name", "chalk-sa"),
					resource.TestCheckResourceAttr("data.chalk_unmanaged_environment.test", "kube_cluster_mode", "BYOC_FULL"),
				),
			},
		},
	})
}

// TestUnmanagedEnvironmentDataSource_NoSecretInSchema asserts the schema does
// not expose online_store_secret.
func TestUnmanagedEnvironmentDataSource_NoSecretInSchema(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	secret := "should-not-appear"
	clusterID := "test-kube-cluster-id"
	server.OnGetEnv().Return(&serverv1.GetEnvResponse{
		Environment: &serverv1.Environment{
			Id:                "test-env-id",
			Name:              "test-env",
			ProjectId:         "test-project-id",
			Managed:           boolPtr(false),
			KubeClusterId:     &clusterID,
			OnlineStoreSecret: &secret,
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testUnmanagedEnvironmentDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_unmanaged_environment.test", "online_store_secret"),
				),
			},
		},
	})
}

// TestUnmanagedEnvironmentDataSource_ManagedMismatch asserts that pointing this
// data source at a managed environment surfaces a clear diagnostic.
func TestUnmanagedEnvironmentDataSource_ManagedMismatch(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	clusterID := "test-kube-cluster-id"
	server.OnGetEnv().Return(&serverv1.GetEnvResponse{
		Environment: &serverv1.Environment{
			Id:            "test-env-id",
			Name:          "test-env",
			ProjectId:     "test-project-id",
			Managed:       boolPtr(true),
			KubeClusterId: &clusterID,
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testUnmanagedEnvironmentDataSourceConfig,
				ExpectError: regexp.MustCompile(`is a managed environment`),
			},
		},
	})
}

func TestUnmanagedEnvironmentDataSource_RpcError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetEnv().ReturnError(connect.NewError(connect.CodeInternal, errors.New("backend exploded")))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testUnmanagedEnvironmentDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read unmanaged environment`),
			},
		},
	})
}
