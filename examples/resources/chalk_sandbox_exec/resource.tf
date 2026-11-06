# Refresh a Coder agent's token whenever Coder rotates it. This script is part
# of the sandbox image; it reads the token from stdin and restarts the agent.
resource "chalk_sandbox_exec" "refresh_coder_token" {
  environment_id = chalk_sandbox.dev.environment_id
  sandbox_id     = chalk_sandbox.dev.id
  command        = ["/usr/local/bin/refresh-coder-agent-token"]
  stdin          = coder_agent.main.token

  triggers = {
    token = sha256(coder_agent.main.token)
  }

  depends_on = [chalk_sandbox.dev]
}
