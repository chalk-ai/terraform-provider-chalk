# A Chalk sandbox: an ephemeral container in the environment's cluster that you
# can exec into with `chalk sandbox exec <id> -- <command>`.
#
# A sandbox has no outbound network access by default — without a network_policy
# even DNS resolution fails — so egress is granted explicitly below.

resource "chalk_sandbox" "workstation" {
  environment_id = "tmnmc9beyujew"
  name           = "debian-workstation"
  image          = "debian:bookworm"

  entrypoint = ["/bin/bash", "-c", "sleep infinity"]

  env = {
    WORKSPACE = "demo"
  }

  resource_limits = {
    cpu    = "2"
    memory = "4Gi"
  }

  volumes = [{
    name       = "home"
    mount_path = "/home/dev"
    type       = "empty_dir"
    size_limit = "20Gi"
  }]

  network_policy = {
    # Full egress. Narrow this to the CIDRs the workload actually needs.
    allowed_routes = [{
      route = "0.0.0.0/0"
    }]

    # The cloud metadata endpoint is never something a sandbox should reach.
    denied_routes = ["169.254.169.254/32"]
  }
}
