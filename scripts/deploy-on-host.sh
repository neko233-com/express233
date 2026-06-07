#!/bin/bash
# 游戏逻辑服 SSH 上一行拉取部署（由运维在机器上或 Ansible 调用）
# 依赖: 已安装 express233-cli
#
# 用法:
#   CENTRAL=http://10.0.0.1:23380 TOKEN=xxx PROJECT=mygame \
#   SERVER_ID=game-logic-042 DEST=/opt/game/042 ./deploy-on-host.sh

set -euo pipefail

: "${CENTRAL:?set CENTRAL}"
: "${TOKEN:?set TOKEN}"
: "${PROJECT:?set PROJECT}"
: "${SERVER_ID:?set SERVER_ID}"
DEST="${DEST:-/opt/game/${SERVER_ID}}"
VERSION="${VERSION:-}"

args=(deploy --server "$CENTRAL" --project "$PROJECT" --server-id "$SERVER_ID" --token "$TOKEN" --dest "$DEST")
[ -n "$VERSION" ] && args+=(--version "$VERSION")

if [ "${PREVIEW:-0}" = "1" ]; then
  express233-cli preview --server "$CENTRAL" --project "$PROJECT" --server-id "$SERVER_ID" --token "$TOKEN" ${VERSION:+--version "$VERSION"}
  exit 0
fi

express233-cli "${args[@]}"
