package provider

import (
	"regexp"
	"testing"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kubeClusterCoreConfig(serverURL, dnsZone string) string {
	return providerConfig(serverURL) + `
resource "chalk_kubernetes_cluster" "cluster" {
  name                = "test-cluster"
  kind                = "EKS_STANDARD"
  cloud_credential_id = "cc-test-id"
  dns_zone            = "` + dnsZone + `"
}
`
}

func kubeClusterFullConfig(serverURL, controllerBlock, maintenanceMode string) string {
	return providerConfig(serverURL) + `
resource "chalk_kubernetes_cluster" "cluster" {
  name = "test-cluster"
  kind = "EKS_STANDARD"

  maintenance_window = {
    mode     = "` + maintenanceMode + `"
    schedule = "0 2 * * *"
    duration = "30m"
  }

  data_plane_redis = {
    managed = {
      memory = "10Gi"
      cpu    = "1"
    }
  }
` + controllerBlock + `
}
`
}

const kubeControllerBlockMedium = `
  data_plane_controller = {
    tier                 = "MEDIUM"
    node_pool            = "open-pool"
    restricted_node_pool = "restricted-pool"
    host_pools = [
      { name = "workers", count = 2, cpu = "4", memory = "8Gi" },
    ]
  }
`

const kubeControllerBlockLarge = `
  data_plane_controller = {
    tier = "LARGE"
    host_pools = [
      { name = "workers", count = 3 },
      { name = "gpu", count = 1, cpu = "8", machine_family = "n2" },
    ]
  }
`

// TestKubernetesClusterResourceCreateRead verifies the core create/read
// round-trip for the unmanaged cluster resource.
func TestKubernetesClusterResourceCreateRead(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: kubeClusterCoreConfig(server.URL, "example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "id", "cluster-cfg-id"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "name", "test-cluster"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "kind", "EKS_STANDARD"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "cloud_credential_id", "cc-test-id"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "dns_zone", "example.com"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "team_id", "team-test-id"),
					func(s *terraform.State) error {
						reqs := server.GetCapturedRequests("CreateCloudComponentCluster")
						require.Len(t, reqs, 1)
						r := reqs[0].(*serverv1.CreateCloudComponentClusterRequest)
						assert.False(t, r.Cluster.GetManaged(), "kubernetes_cluster must be unmanaged")
						assert.Equal(t, "test-cluster", r.Cluster.GetSpec().GetName())
						assert.Equal(t, "example.com", r.Cluster.GetSpec().GetDnsZone())
						return nil
					},
				),
			},
		},
	})
}

// TestKubernetesClusterResourceUpdateDnsZone verifies that mutating dns_zone
// issues an update and reads back the new value.
func TestKubernetesClusterResourceUpdateDnsZone(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: kubeClusterCoreConfig(server.URL, "example.com"),
				Check:  resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "dns_zone", "example.com"),
			},
			{
				Config: kubeClusterCoreConfig(server.URL, "updated.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "dns_zone", "updated.example.com"),
					func(s *terraform.State) error {
						reqs := server.GetCapturedRequests("UpdateCloudComponentCluster")
						require.GreaterOrEqual(t, len(reqs), 1)
						spec := reqs[len(reqs)-1].(*serverv1.UpdateCloudComponentClusterRequest).Cluster.GetSpec()
						assert.Equal(t, "updated.example.com", spec.GetDnsZone())
						return nil
					},
				),
			},
		},
	})
}

// TestKubernetesClusterResourceDeleteIsNoOp verifies that removing the resource
// only drops it from state and never calls DeleteCloudComponentCluster, since
// the underlying cluster is unmanaged.
func TestKubernetesClusterResourceDeleteIsNoOp(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: kubeClusterCoreConfig(server.URL, "example.com")},
			{
				Config: providerConfig(server.URL),
				Check: func(s *terraform.State) error {
					assert.Empty(t, server.GetCapturedRequests("DeleteCloudComponentCluster"),
						"unmanaged cluster delete must not call the server")
					return nil
				},
			},
		},
	})
}

// TestKubernetesClusterResourceImportState verifies import by cluster id.
func TestKubernetesClusterResourceImportState(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: kubeClusterCoreConfig(server.URL, "example.com")},
			{
				ResourceName:      "chalk_kubernetes_cluster.cluster",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestKubernetesClusterResourceConfigRoundTrip verifies that the shared
// cluster-level config blocks are sent on create, read back consistently, and
// mutated on update.
func TestKubernetesClusterResourceConfigRoundTrip(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: kubeClusterFullConfig(server.URL, kubeControllerBlockMedium, maintenanceModeCustom),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "maintenance_window.mode", "CUSTOM"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "maintenance_window.schedule", "0 2 * * *"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_redis.managed.memory", "10Gi"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_redis.managed.cpu", "1"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.tier", "MEDIUM"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.node_pool", "open-pool"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.restricted_node_pool", "restricted-pool"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.host_pools.#", "1"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.host_pools.0.name", "workers"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.host_pools.0.count", "2"),
					func(s *terraform.State) error {
						reqs := server.GetCapturedRequests("CreateCloudComponentCluster")
						require.Len(t, reqs, 1)
						spec := reqs[0].(*serverv1.CreateCloudComponentClusterRequest).Cluster.GetSpec()
						assert.Equal(t, serverv1.MaintenanceWindow_MODE_CUSTOM, spec.GetMaintenanceWindow().GetMode())
						assert.Equal(t, "MANAGED", spec.GetDataPlaneRedis().GetKind())
						assert.Equal(t, serverv1.DataplaneController_TIER_MEDIUM, spec.GetDataplaneController().GetTier())
						require.Len(t, spec.GetDataplaneController().GetHostPools(), 1)
						assert.Equal(t, int32(2), spec.GetDataplaneController().GetHostPools()[0].GetCount())
						// available_tiers is output-only and must never be sent.
						assert.Nil(t, spec.GetDataplaneController().GetAvailableTiers())
						return nil
					},
				),
			},
			{
				Config: kubeClusterFullConfig(server.URL, kubeControllerBlockLarge, maintenanceModeUnrestricted),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "maintenance_window.mode", "UNRESTRICTED"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.tier", "LARGE"),
					resource.TestCheckNoResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.node_pool"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.host_pools.#", "2"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.host_pools.1.name", "gpu"),
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.host_pools.1.machine_family", "n2"),
				),
			},
		},
	})
}

// TestKubernetesClusterResourceConfigClearOnUpdate verifies that removing the
// config blocks from a cluster that had them clears them server-side (the update
// sends nil blocks), the state drops them, and re-applying is a no-op.
func TestKubernetesClusterResourceConfigClearOnUpdate(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	withConfig := kubeClusterFullConfig(server.URL, kubeControllerBlockMedium, maintenanceModeCustom)
	cleared := providerConfig(server.URL) + `
resource "chalk_kubernetes_cluster" "cluster" {
  name = "test-cluster"
  kind = "EKS_STANDARD"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: withConfig,
				Check:  resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.tier", "MEDIUM"),
			},
			{
				Config: cleared,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("chalk_kubernetes_cluster.cluster", "maintenance_window.mode"),
					resource.TestCheckNoResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_redis.managed.memory"),
					resource.TestCheckNoResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.tier"),
					func(s *terraform.State) error {
						reqs := server.GetCapturedRequests("UpdateCloudComponentCluster")
						require.GreaterOrEqual(t, len(reqs), 1)
						spec := reqs[len(reqs)-1].(*serverv1.UpdateCloudComponentClusterRequest).Cluster.GetSpec()
						assert.Nil(t, spec.GetMaintenanceWindow(), "update should clear maintenance_window")
						assert.Nil(t, spec.GetDataPlaneRedis(), "update should clear data_plane_redis")
						assert.Nil(t, spec.GetDataplaneController(), "update should clear data_plane_controller")
						return nil
					},
				),
			},
			{
				// Clearing must be stable: re-applying yields an empty plan.
				Config:   cleared,
				PlanOnly: true,
			},
		},
	})
}

// TestKubernetesClusterResourceDataPlaneRedisSelfHosted verifies the self_hosted
// branch of the data_plane_redis one-of maps to kind SELF_HOSTED.
func TestKubernetesClusterResourceDataPlaneRedisSelfHosted(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	config := providerConfig(server.URL) + `
resource "chalk_kubernetes_cluster" "cluster" {
  name = "test-cluster"
  kind = "EKS_STANDARD"

  data_plane_redis = {
    self_hosted = {
      cloud_secret_name = "redis-creds"
    }
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_redis.self_hosted.cloud_secret_name", "redis-creds"),
					resource.TestCheckNoResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_redis.managed.memory"),
					func(s *terraform.State) error {
						reqs := server.GetCapturedRequests("CreateCloudComponentCluster")
						require.Len(t, reqs, 1)
						redis := reqs[0].(*serverv1.CreateCloudComponentClusterRequest).Cluster.GetSpec().GetDataPlaneRedis()
						assert.Equal(t, "SELF_HOSTED", redis.GetKind())
						assert.Equal(t, "redis-creds", redis.GetCloudSecretName())
						return nil
					},
				),
			},
		},
	})
}

// TestKubernetesClusterResourceDataPlaneRedisRequiresExactlyOne verifies that
// setting both managed and self_hosted is rejected at plan time.
func TestKubernetesClusterResourceDataPlaneRedisRequiresExactlyOne(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	config := providerConfig(server.URL) + `
resource "chalk_kubernetes_cluster" "cluster" {
  name = "test-cluster"
  kind = "EKS_STANDARD"

  data_plane_redis = {
    managed     = {}
    self_hosted = { cloud_secret_name = "redis-creds" }
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
			},
		},
	})
}

// TestKubernetesClusterResourceDataPlaneControllerRejectsEmpty verifies that an
// empty data_plane_controller block is rejected (it would otherwise drift to
// null after apply).
func TestKubernetesClusterResourceDataPlaneControllerRejectsEmpty(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	config := providerConfig(server.URL) + `
resource "chalk_kubernetes_cluster" "cluster" {
  name = "test-cluster"
  kind = "EKS_STANDARD"

  data_plane_controller = {}
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination|At least one attribute`),
			},
		},
	})
}

// TestKubernetesClusterResourceHostPoolsRejectsEmptyList verifies that an empty
// host_pools list is rejected in favor of omitting the attribute.
func TestKubernetesClusterResourceHostPoolsRejectsEmptyList(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	config := providerConfig(server.URL) + `
resource "chalk_kubernetes_cluster" "cluster" {
  name = "test-cluster"
  kind = "EKS_STANDARD"

  data_plane_controller = {
    tier       = "SMALL"
    host_pools = []
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`(?s)must contain at least 1`),
			},
		},
	})
}

// TestKubernetesClusterResourceMaintenanceEmptyStringRejected verifies that an
// empty schedule is rejected at plan time. schedule/duration are plain proto
// strings with no presence bit, so "" is indistinguishable from unset and would
// drift; it's forbidden rather than allowed.
func TestKubernetesClusterResourceMaintenanceEmptyStringRejected(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	config := providerConfig(server.URL) + `
resource "chalk_kubernetes_cluster" "cluster" {
  name = "test-cluster"
  kind = "EKS_STANDARD"

  maintenance_window = {
    mode     = "CUSTOM"
    schedule = ""
    duration = "30m"
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`(?s)string length must be at least 1`),
			},
		},
	})
}

// TestKubernetesClusterResourceOptionalEmptyStringRoundTrip verifies that an
// optional (pointer) string set to an explicit "" round-trips without drift.
// Presence-aware flatten distinguishes "unset" (null) from "explicitly empty"
// (""); a value-based flatten would collapse "" to null and error.
func TestKubernetesClusterResourceOptionalEmptyStringRoundTrip(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	config := providerConfig(server.URL) + `
resource "chalk_kubernetes_cluster" "cluster" {
  name = "test-cluster"
  kind = "EKS_STANDARD"

  data_plane_controller = {
    tier      = "SMALL"
    node_pool = ""
  }
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.node_pool", ""),
			},
			{
				// Re-applying must be a no-op: "" must not drift to null.
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// TestKubernetesClusterResourceConfigOmittedNoDrift verifies that a cluster with
// no data_plane_controller block does not drift even though the server hydrates
// a non-nil (but empty) controller on every response.
func TestKubernetesClusterResourceConfigOmittedNoDrift(t *testing.T) {
	t.Parallel()

	server := setupClusterConfigServer(t, false)

	config := providerConfig(server.URL) + `
resource "chalk_kubernetes_cluster" "cluster" {
  name = "test-cluster"
  kind = "EKS_STANDARD"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_controller.tier"),
					resource.TestCheckNoResourceAttr("chalk_kubernetes_cluster.cluster", "maintenance_window.mode"),
					resource.TestCheckNoResourceAttr("chalk_kubernetes_cluster.cluster", "data_plane_redis.managed.memory"),
				),
			},
			{
				// Re-applying the identical config must produce an empty plan: the
				// hydrated controller must not surface as a phantom object.
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}
