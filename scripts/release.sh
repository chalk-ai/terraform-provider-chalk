#!/usr/bin/env bash
set -euo pipefail

# Bump level: major | minor | patch (default: patch).
#
# Pick per semver: patch for fixes, minor for backward-compatible changes,
# major for backward-incompatible ones. Releases are picked up automatically by
# users pinned with `~>`.
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

if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree must be clean before releasing" >&2
  exit 1
fi

go run ./tools/genchangelog --provider-dir . --check-snapshot "${NEXT_TAG}"
go run ./tools/genchangelog --provider-dir . --check

read -r -p "Push tag and create terraform provider release? [y/N] " response
case "$response" in
  [yY][eE][sS]|[yY]) ;;
  *) exit 0 ;;
esac

git tag -a "${NEXT_TAG}" -m "Release terraform-provider-chalk ${NEXT_TAG}"
git push origin "${NEXT_TAG}"
gh release create "${NEXT_TAG}" --generate-notes
