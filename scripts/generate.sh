#!/usr/bin/env bash
# Generate testdata/schema/*.json into tests/generated/.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "${root}"
if [[ -x /home/leo/go/bin/go ]]; then
  export PATH="/home/leo/go/bin:${PATH}"
fi
mkdir -p tests/generated "${root}/temp"
go build -o "${root}/temp/gld-kilngen-go" ./cmd/gld-kilngen-go
shopt -s nullglob
for schema in testdata/schema/*.json; do
  "${root}/temp/gld-kilngen-go" --schema "${schema}" --out tests/generated
done
