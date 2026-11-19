#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
echo "== go test ./... =="
CGO_ENABLED=0 go test ./... -count=1 "$@"
echo "OK"
