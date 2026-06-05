#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

VERSION="${VERSION:-0.0.0-local}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo local)}"
LDFLAGS="-s -w \
  -X github.com/neko233-com/express233/internal/version.Version=${VERSION} \
  -X github.com/neko233-com/express233/internal/version.Commit=${COMMIT}"

echo "== build-all =="
mkdir -p bin
go build -ldflags "$LDFLAGS" -o bin/express233 ./cmd/express233
go build -ldflags "$LDFLAGS" -o bin/express233-server ./cmd/express233-server
echo "OK: bin/express233 bin/express233-server"
