# A host pool scoped to an environment. Host pools provide the hosts that Chalk
# Compute workloads run on.
#
# Pools either hold a fixed number of hosts (min_hosts = max_hosts) or scale down
# to zero when idle (min_hosts = 0). idle_timeout is required in the scaling case
# and must not be set for fixed-size pools.

resource "chalk_environment_host_pool" "workers" {
  environment_id = chalk_managed_environment.example.id

  name         = "workers"
  min_hosts    = 0
  max_hosts    = 4
  idle_timeout = "5m"
  cpu          = "4"
  memory       = "8Gi"
}

# A fixed-size pool pinned to a machine family.
resource "chalk_environment_host_pool" "gpu" {
  environment_id = chalk_managed_environment.example.id

  name           = "gpu"
  min_hosts      = 2
  max_hosts      = 2
  cpu            = "8"
  memory         = "32Gi"
  machine_family = "n2"
}
