#!/usr/bin/env bash
set -euo pipefail

LAST_TAG=$(gh release view --json tagName --jq '.tagName')
NEXT_TAG=$(echo "${LAST_TAG}" | awk -F. -v OFS=. 'NF==1{print ++$NF}; NF>1{$NF=$NF+1; print}')
echo "Bumping from ${LAST_TAG} -> ${NEXT_TAG}"

read -r -p "Push tag and create terraform provider release? [y/N] " response
case "$response" in
  [yY][eE][sS]|[yY]) ;;
  *) exit 0 ;;
esac

git tag -a "${NEXT_TAG}" -m "Release terraform-provider-chalk ${NEXT_TAG}"
git push origin "${NEXT_TAG}"
gh release create "${NEXT_TAG}" --generate-notes
