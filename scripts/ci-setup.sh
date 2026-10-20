#!/usr/bin/env bash
# Confirm Go is on PATH. GitHub Actions installs it with actions/setup-go first.
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  echo "go is not on PATH" >&2
  exit 1
fi
go version
