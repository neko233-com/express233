#!/usr/bin/env bash
# 本地模拟 GitHub Actions：vet、测试、构建、冒烟
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-0.0.0-local}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo local)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
LDFLAGS="-s -w \
  -X github.com/neko233-com/express233/internal/version.Version=${VERSION} \
  -X github.com/neko233-com/express233/internal/version.Commit=${COMMIT} \
  -X github.com/neko233-com/express233/internal/version.Date=${DATE}"

echo "== vet =="
go vet ./...

echo "== test =="
go test ./... -count=1

echo "== build =="
mkdir -p bin
go build -ldflags "$LDFLAGS" -o bin/express233 ./cmd/express233
go build -ldflags "$LDFLAGS" -o bin/express233-server ./cmd/express233-server

if [[ -f scripts/ci-smoke.sh ]]; then
  echo "== smoke =="
  bash scripts/ci-smoke.sh
fi

echo "deploy-github: OK (binaries in bin/)"
