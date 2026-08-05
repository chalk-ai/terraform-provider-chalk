# A host pool scoped to a cluster, shared by every environment on that cluster.
#
# Pools either hold a fixed number of hosts (min_hosts = max_hosts) or scale down
# to zero when idle (min_hosts = 0). idle_timeout is required in the scaling case
# and must not be set for fixed-size pools.

resource "chalk_cluster_host_pool" "workers" {
  cluster_id = chalk_managed_cluster.example.id

  name         = "workers"
  min_hosts    = 0
  max_hosts    = 8
  idle_timeout = "10m"
  cpu          = "4"
  memory       = "8Gi"
}
