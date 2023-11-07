#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$(mktemp -t oapi-ginx.XXXXXX)"
trap 'rm -f "$BIN"' EXIT

cd "$ROOT"

go build -o "$BIN" ./cmd/oapi-ginx

for dir in internal/codegen/e2etest/openapi-3.*/code/*; do
  if [[ -d "$dir" ]]; then
    echo "generating $dir"
    (
      cd "$dir"
      rm -f -- *.gen.go
      "$BIN" -c oapi-ginx.yaml
    )
  fi
done