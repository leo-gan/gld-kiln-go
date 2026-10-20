#!/usr/bin/env bash
# Unit tests, including the golden cases and generated structs.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "${root}"

if [[ -x /home/leo/go/bin/go ]]; then
  export PATH="/home/leo/go/bin:${PATH}"
fi
goroot="$(go env GOROOT)"
if [[ -z "${GOPATH:-}" || "${GOPATH}" == "${goroot}" ]]; then
  export GOPATH="${HOME}/gopath"
  mkdir -p "${GOPATH}"
fi

go test ./...
