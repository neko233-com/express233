#!/bin/bash
# 备份 express233-server 数据目录（SQLite + server.yaml + 版本文件）
set -euo pipefail

SRC="${1:-${EXPRESS233_DATA:-$HOME/.express233-server}}"
DEST="${2:-./express233-backup-$(date +%Y%m%d-%H%M%S)}"

if [ ! -d "$SRC" ]; then
  echo "source not found: $SRC" >&2
  exit 1
fi

mkdir -p "$DEST"
tar -czf "${DEST}.tar.gz" -C "$(dirname "$SRC")" "$(basename "$SRC")"
echo "backup written to ${DEST}.tar.gz"
