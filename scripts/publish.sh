#!/usr/bin/env bash
# Go ships as a module tag. There is no package registry upload.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "${root}"

version=""
if [[ -f VERSION ]]; then
  version="$(tr -d '[:space:]' < VERSION)"
fi
echo "Go ships as a module tag."
if [[ -n "${version}" ]]; then
  echo "Install with: go get github.com/leo-gan/gld-kiln-go@v${version}"
else
  echo "VERSION is missing; the module tag is still the release name."
fi
if [[ -z "${GO_REGISTRY_TOKEN:-}" ]]; then
  echo "GO_REGISTRY_TOKEN is unset. Nothing was published to a package registry."
fi
exit 0
