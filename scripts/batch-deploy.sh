#!/bin/bash
# batch-deploy.sh — 同机多 game-server 批量安全部署
#
# 用法:
#   batch-deploy.sh --servers "game-logic-01,game-logic-02,game-logic-03" [--version 1.0.0]
#   batch-deploy.sh --file servers.txt [--version 1.0.0]
#
# servers.txt 格式 (每行一个 server_id，可选指定目录):
#   game-logic-01
#   game-logic-02
#   game-logic-03
#
# 每个 server_id 独立:
#   - 临时目录: GAME_ROOT/.tmp/{server_id}/
#   - 最终目录: GAME_ROOT/{server_id}/
#   - 日志目录: GAME_ROOT/{server_id}/logs/
#   - PID 文件: GAME_ROOT/{server_id}/run/server.pid
#
# 部署策略: 串行执行（避免同时停多个服），单个失败不阻断其余。
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SAFE_DEPLOY="${SCRIPT_DIR}/safe-deploy.sh"
GAME_ROOT="${GAME_ROOT:-/opt/game-servers}"
VERSION_FLAG=""
SERVERS=""
FAILURES=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --servers)   SERVERS="$2"; shift 2 ;;
    --file)      SERVERS=$(cat "$2" | tr '\n' ','); shift 2 ;;
    --version)   VERSION_FLAG="--version $2"; shift 2 ;;
    --root)      GAME_ROOT="$2"; shift 2 ;;
    -h|--help)   sed -n '2,15p' "$0"; exit 0 ;;
    *) echo "unknown: $1"; exit 1 ;;
  esac
done

if [[ -z "$SERVERS" ]]; then
  echo "error: --servers or --file required"
  exit 1
fi

IFS=',' read -ra SERVER_LIST <<< "$SERVERS"
TOTAL=${#SERVER_LIST[@]}
echo "=========================================="
echo " batch deploy: ${TOTAL} server(s)"
echo " root: ${GAME_ROOT}"
echo "=========================================="

i=0
for sid in "${SERVER_LIST[@]}"; do
  sid=$(echo "$sid" | xargs)  # trim whitespace
  [[ -z "$sid" ]] && continue
  i=$((i + 1))
  echo ""
  echo "────────── [$i/$TOTAL] $sid ──────────"
  if GAME_ROOT="$GAME_ROOT" bash "$SAFE_DEPLOY" \
      --server-id "$sid" \
      $VERSION_FLAG; then
    echo "  ✓ $sid deployed"
  else
    echo "  ✗ $sid FAILED (exit $?)"
    FAILURES+=("$sid")
  fi
done

echo ""
echo "=========================================="
echo " Results: $((TOTAL - ${#FAILURES[@]}))/$TOTAL succeeded"
if [[ ${#FAILURES[@]} -gt 0 ]]; then
  echo " Failed: ${FAILURES[*]}"
  exit 1
fi
echo "=========================================="
