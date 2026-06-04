# shellcheck shell=bash
# 定位仓库根目录（scripts/lib 的上级的上级）
express233_root() {
  local here
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
  printf '%s' "$here"
}
