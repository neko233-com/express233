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

stop_previous_server() {
  local port="$1"
  if ! curl -sf --max-time 2 "http://127.0.0.1:${port}/healthz" | grep -qxF ok; then
    return 0
  fi
  local pids=()
  if command -v lsof >/dev/null 2>&1; then
    while IFS= read -r pid; do
      [[ -n "$pid" ]] && pids+=("$pid")
    done < <(lsof -t -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null || true)
  fi
  for pid in "${pids[@]}"; do
    local name
    name="$(ps -p "$pid" -o comm= 2>/dev/null || true)"
    if [[ "$name" != *express233-server* ]]; then
      continue
    fi
    echo "[停止] express233-server PID ${pid}"
    if command -v unicli >/dev/null 2>&1; then
      unicli kill "$pid" >/dev/null 2>&1 || kill "$pid" 2>/dev/null || true
    else
      kill "$pid" 2>/dev/null || true
    fi
  done
  sleep 1
}

if [[ -z "${EXPRESS233_NO_KILL:-}" ]]; then
  stop_previous_server "$PORT"
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
