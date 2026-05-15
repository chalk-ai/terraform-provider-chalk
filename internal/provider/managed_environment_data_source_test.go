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

const testManagedEnvironmentDataSourceConfig = `
data "chalk_managed_environment" "test" {
  id = "test-env-id"
}
`

func boolPtr(b bool) *bool { return &b }

func TestManagedEnvironmentDataSource_AllAttributes(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	clusterID := "test-kube-cluster-id"
	jobNs := "chalk-ns"
	serviceURL := "https://env.chalk.ai"
	storeKind := "redis"

	server.OnGetEnv().Return(&serverv1.GetEnvResponse{
		Environment: &serverv1.Environment{
			Id:               "test-env-id",
			Name:             "test-env",
			ProjectId:        "test-project-id",
			Managed:          boolPtr(true),
			KubeClusterId:    &clusterID,
			KubeJobNamespace: &jobNs,
			ServiceUrl:       &serviceURL,
			OnlineStoreKind:  &storeKind,
			AdditionalEnvVars: map[string]string{
				"FOO": "bar",
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testManagedEnvironmentDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_managed_environment.test", "id", "test-env-id"),
					resource.TestCheckResourceAttr("data.chalk_managed_environment.test", "name", "test-env"),
					resource.TestCheckResourceAttr("data.chalk_managed_environment.test", "project_id", "test-project-id"),
					resource.TestCheckResourceAttr("data.chalk_managed_environment.test", "kube_cluster_id", "test-kube-cluster-id"),
					resource.TestCheckResourceAttr("data.chalk_managed_environment.test", "kube_job_namespace", "chalk-ns"),
					resource.TestCheckResourceAttr("data.chalk_managed_environment.test", "service_url", "https://env.chalk.ai"),
					resource.TestCheckResourceAttr("data.chalk_managed_environment.test", "online_store_kind", "redis"),
					resource.TestCheckResourceAttr("data.chalk_managed_environment.test", "additional_env_vars.FOO", "bar"),
				),
			},
		},
	})
}

// TestManagedEnvironmentDataSource_NoSecretInSchema ensures the schema does
// not expose the online_store_secret field — a deliberate design choice for
// new data sources.
func TestManagedEnvironmentDataSource_NoSecretInSchema(t *testing.T) {
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
			Managed:           boolPtr(true),
			KubeClusterId:     &clusterID,
			OnlineStoreSecret: &secret,
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testManagedEnvironmentDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_managed_environment.test", "online_store_secret"),
				),
			},
		},
	})
}

func TestManagedEnvironmentDataSource_RpcError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetEnv().ReturnError(connect.NewError(connect.CodeInternal, errors.New("backend exploded")))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testManagedEnvironmentDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read managed environment`),
			},
		},
	})
}
