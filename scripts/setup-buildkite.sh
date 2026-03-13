#!/usr/bin/env bash
set -euo pipefail

# Creates (or updates) Buildkite pipelines for terraform-provider-chalk.
# Requires BUILDKITE_API_TOKEN to be set in the environment.
#
# IMPORTANT: Pipeline configurations are stored statically in Buildkite, not loaded from
# the repo on each build. Changes to pipeline YAML files only take effect after
# re-running this script.
#
# Usage:
#   BUILDKITE_API_TOKEN=<token> ./scripts/setup-buildkite.sh

ORG="chalk"
CLUSTER_ID="7b9e4371-390a-4f64-88b8-b281a34c0843"
TEAM_UUID="ee978f38-5c01-44a4-93d9-e6e36dfdd754"
API="https://api.buildkite.com/v2/organizations/${ORG}"
REPO="git@github.com:chalk-ai/terraform-provider-chalk.git"

: "${BUILDKITE_API_TOKEN:?BUILDKITE_API_TOKEN is not set}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

upsert_pipeline() {
  local pipeline="$1"
  local config_file="$2"
  local config
  config="$(cat "${config_file}")"

  local status
  status=$(curl -sS -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${BUILDKITE_API_TOKEN}" \
    "${API}/pipelines/${pipeline}")

  if [[ "$status" == "200" ]]; then
    echo "Pipeline '${pipeline}' exists, updating configuration..."
    curl -sS -X PATCH \
      -H "Authorization: Bearer ${BUILDKITE_API_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "$(jq -n --arg config "$config" '{configuration: $config}')" \
      "${API}/pipelines/${pipeline}" | jq '{slug, name, web_url}'
  else
    echo "Pipeline '${pipeline}' not found, creating..."
    curl -sS -X POST \
      -H "Authorization: Bearer ${BUILDKITE_API_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "$(jq -n \
        --arg name "${pipeline}" \
        --arg repo "${REPO}" \
        --arg config "$config" \
        --arg cluster_id "${CLUSTER_ID}" \
        --arg team_uuid "${TEAM_UUID}" \
        '{
          name: $name,
          repository: $repo,
          default_branch: "main",
          cluster_id: $cluster_id,
          configuration: $config,
          team_uuids: [$team_uuid]
        }')" \
      "${API}/pipelines" | jq '{slug, name, web_url, cluster_id}'
    echo ""
    echo "NOTE: If the GitHub webhook was not automatically created, see:"
    echo "  https://buildkite.com/chalk/${pipeline}/settings/setup/github"
  fi
}

upsert_pipeline "terraform-provider-chalk-pr"      "${SCRIPT_DIR}/buildkite-pipeline-pr.yml"
echo ""
upsert_pipeline "terraform-provider-chalk-release" "${SCRIPT_DIR}/buildkite-pipeline-release.yml"