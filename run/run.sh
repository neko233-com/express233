#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export EXPRESS233_DATA="${EXPRESS233_DATA:-$ROOT/.data}"
mkdir -p "$EXPRESS233_DATA"
echo "express233-server on :23380 (data: $EXPRESS233_DATA)"
exec go run ./cmd/express233-server -addr :23380 -data "$EXPRESS233_DATA"
