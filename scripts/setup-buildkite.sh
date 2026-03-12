#!/usr/bin/env bash
set -euo pipefail

# Creates (or updates) the Buildkite PR validation pipeline for terraform-provider-chalk.
# Requires BUILDKITE_API_TOKEN to be set in the environment.
#
# Usage:
#   BUILDKITE_API_TOKEN=<token> ./scripts/setup-buildkite.sh          # create
#   BUILDKITE_API_TOKEN=<token> ./scripts/setup-buildkite.sh update   # update config only

ORG="chalk"
CLUSTER_ID="7b9e4371-390a-4f64-88b8-b281a34c0843"
TEAM_UUID="ee978f38-5c01-44a4-93d9-e6e36dfdd754"
PIPELINE="terraform-provider-chalk-pr"
API="https://api.buildkite.com/v2/organizations/${ORG}"

: "${BUILDKITE_API_TOKEN:?BUILDKITE_API_TOKEN is not set}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG="$(cat "${SCRIPT_DIR}/../.buildkite/pipeline.yml")"

STATUS=$(curl -sS -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer ${BUILDKITE_API_TOKEN}" \
  "${API}/pipelines/${PIPELINE}")

if [[ "$STATUS" == "200" ]]; then
  echo "Pipeline exists, updating configuration..."
  curl -sS -X PATCH \
    -H "Authorization: Bearer ${BUILDKITE_API_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$(jq -n --arg config "$CONFIG" '{configuration: $config}')" \
    "${API}/pipelines/${PIPELINE}" | jq '{slug, name, web_url}'
else
  echo "Pipeline not found, creating..."
  curl -sS -X POST \
    -H "Authorization: Bearer ${BUILDKITE_API_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$(jq -n \
      --arg name "${PIPELINE}" \
      --arg repo "git@github.com:chalk-ai/terraform-provider-chalk.git" \
      --arg config "$CONFIG" \
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
  echo "  https://buildkite.com/chalk/terraform-provider-chalk-pr/settings/setup/github"
fi
