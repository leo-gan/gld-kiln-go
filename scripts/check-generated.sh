#!/usr/bin/env bash
# Regenerate into a temp directory and diff against tests/generated.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "${root}"
if [[ -x /home/leo/go/bin/go ]]; then
  export PATH="/home/leo/go/bin:${PATH}"
fi
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
go build -o "${tmp}/gld-kilngen-go" ./cmd/gld-kilngen-go
shopt -s nullglob
for schema in testdata/schema/*.json; do
  "${tmp}/gld-kilngen-go" --schema "${schema}" --out "${tmp}/out"
done
status=0
for src in "${tmp}/out"/*.go; do
  base="$(basename "${src}")"
  if ! diff -u "tests/generated/${base}" "${src}"; then
    status=1
  fi
done
for src in tests/generated/*.go; do
  base="$(basename "${src}")"
  case "${base}" in
    *_test.go) continue ;;
  esac
  if [[ ! -f "${tmp}/out/${base}" ]]; then
    echo "missing generated file ${base}" >&2
    status=1
  fi
done
exit "${status}"
