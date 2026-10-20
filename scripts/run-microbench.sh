#!/usr/bin/env bash
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
go test -bench='Benchmark(Struct|Tensor)' -benchmem -count=1 -run '^$' .
