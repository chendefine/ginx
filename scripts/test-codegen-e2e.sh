#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

"$ROOT/scripts/generate-codegen-e2e.sh"

cd "$ROOT"
go test ./internal/codegen/... -count=1 "$@"