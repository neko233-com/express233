#!/bin/bash
# 从 backup-data.sh 生成的 tar.gz 恢复数据目录
set -euo pipefail

ARCHIVE="${1:?usage: restore-data.sh <backup.tar.gz> [target_dir]}"
TARGET="${2:-${EXPRESS233_DATA:-$HOME/.express233-server}}"

mkdir -p "$TARGET"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

tar -xzf "$ARCHIVE" -C "$TMP"
# 归档内为单层目录名
SRC=$(find "$TMP" -mindepth 1 -maxdepth 1 -type d | head -1)
if [ -z "$SRC" ]; then
  echo "invalid archive layout" >&2
  exit 1
fi

echo "restoring $SRC -> $TARGET"
rsync -a --delete "$SRC/" "$TARGET/"
echo "restore complete: $TARGET"
