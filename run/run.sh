#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export EXPRESS233_DATA="${EXPRESS233_DATA:-$ROOT/.data}"
export EXPRESS233_ADDR="${EXPRESS233_ADDR:-127.0.0.1:23380}"
export EXPRESS233_WEB_DIR="${EXPRESS233_WEB_DIR:-$ROOT/internal/api/web}"
mkdir -p "$EXPRESS233_DATA"

HOST="${HOST:-127.0.0.1}"
PORT="${PORT:-23380}"
if [[ "$EXPRESS233_ADDR" == *:* ]]; then
  HOST="${EXPRESS233_ADDR%%:*}"
  PORT="${EXPRESS233_ADDR##*:}"
fi

echo ""
echo "-----------------"
echo "访问地址 = http://${HOST}:${PORT}"
echo "数据目录 = ${EXPRESS233_DATA}"
echo "静态热重载 = ${EXPRESS233_WEB_DIR}"
echo "默认账号 = root / root"
echo "-----------------"
echo ""

exec go run ./cmd/express233-server -addr "$EXPRESS233_ADDR" -data "$EXPRESS233_DATA" "$@"
