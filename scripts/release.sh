#!/usr/bin/env bash
set -euo pipefail

# Bump level: major | minor | patch (default: patch).
#
# Use `major` for any release that removes or renames public resource
# attributes, or otherwise breaks existing configurations. Patch and minor
# releases are picked up automatically by users pinned with `~>`, so a breaking
# change MUST bump the major version.
# https://developer.hashicorp.com/terraform/plugin/framework/deprecations#provider-attribute-removal
BUMP="${1:-patch}"

LAST_TAG=$(gh release view --json tagName --jq '.tagName')

# Preserve an optional leading `v`, then split into major.minor.patch.
prefix=""
version="${LAST_TAG}"
if [[ "${version}" == v* ]]; then
  prefix="v"
  version="${version#v}"
fi
IFS=. read -r major minor patch <<<"${version}"
major="${major:-0}"
minor="${minor:-0}"
patch="${patch:-0}"

case "${BUMP}" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
  *) echo "usage: release.sh [major|minor|patch]" >&2; exit 2 ;;
esac

NEXT_TAG="${prefix}${major}.${minor}.${patch}"
echo "Bumping from ${LAST_TAG} -> ${NEXT_TAG} (${BUMP})"

read -r -p "Push tag and create terraform provider release? [y/N] " response
case "$response" in
  [yY][eE][sS]|[yY]) ;;
  *) exit 0 ;;
esac

git tag -a "${NEXT_TAG}" -m "Release terraform-provider-chalk ${NEXT_TAG}"
git push origin "${NEXT_TAG}"
gh release create "${NEXT_TAG}" --generate-notes
