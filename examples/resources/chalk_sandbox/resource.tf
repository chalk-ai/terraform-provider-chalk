# A Chalk sandbox: an ephemeral container in the environment's cluster that you
# can exec into with `chalk sandbox exec <id> -- <command>`.
#
# A sandbox has no outbound network access by default — without a network_policy
# even DNS resolution fails — so egress is granted explicitly below.

resource "chalk_sandbox" "workstation" {
  environment_id = "tmnmc9beyujew"
  name           = "debian-workstation"
  image          = "debian:bookworm"
  # Set desired_state to opt into lifecycle management. When omitted, Terraform
  # leaves out-of-band suspend and resume changes alone.
  desired_state = "suspended"
  # Coder pattern: data.coder_workspace.me.start_count == 1 ? "running" : "suspended"

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

# GCP workload identity federation without a Google key in the sandbox. The
# sandbox runs as its own workload principal; Chalk mints an OIDC token for the
# workload identity provider's audience, writes it to a file, points
# GCP_OIDC_TOKEN_FILE at that file, and rotates it before expiry. Inside the
# sandbox, an `external_account` credential config whose
# credential_source.file is that path lets Google client libraries exchange it.
resource "chalk_sandbox" "gcp_wif" {
  environment_id = "tmnmc9beyujew"
  image          = "google/cloud-sdk:slim"

  chalk_workload_identity = {
    mode = "workload_principal"
    oidc_token_files = [{
      audience = "//iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL/providers/PROVIDER"
      env_var  = "GCP_OIDC_TOKEN_FILE"
    }]
  }

  # The token exchange needs egress to sts.googleapis.com and the Google APIs.
  network_policy = {
    allowed_routes = [{ route = "0.0.0.0/0" }]
    denied_routes  = ["169.254.169.254/32"]
  }
}
