#!/bin/bash
# safe-deploy.sh — 安全部署游戏逻辑服（单实例）
#
# 用法:
#   safe-deploy.sh --server-id game-logic-01 [--version 1.0.0] [--dry-run]
#
# 配置来源: ~/.express233/config.yaml 或环境变量
#   EXPRESS233_SERVER / EXPRESS233_TOKEN / EXPRESS233_PROJECT / EXPRESS233_SERVER_ID
#
# 目录布局 (所有路径均可通过环境变量覆盖):
#   最终目录:  GAME_ROOT/{server_id}/          (默认 /opt/game-servers/{server_id}/)
#   临时目录:  GAME_ROOT/.tmp/{server_id}/     (拉取暂存，部署后自动清理)
#   日志目录:  GAME_ROOT/{server_id}/logs/     (部署不触碰，持久保留)
#   PID 文件:  GAME_ROOT/{server_id}/run/server.pid
#   备份目录:  GAME_ROOT/.backup/{server_id}/  (可选，--backup 开启)
#
set -euo pipefail

# ═══════════════ 配置 ═══════════════
GAME_ROOT="${GAME_ROOT:-/opt/game-servers}"
EXPRESS233_BIN="${EXPRESS233_BIN:-express233-cli}"
STOP_TIMEOUT="${STOP_TIMEOUT:-10}"        # 等待进程退出的秒数
DRY_RUN=false
BACKUP=false
VERSION_ARGS=()

# ═══════════════ 参数解析 ═══════════════
SERVER_ID="${EXPRESS233_SERVER_ID:-}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-id)  SERVER_ID="$2"; shift 2 ;;
    --version)    VERSION_ARGS=(--version "$2"); shift 2 ;;
    --dry-run)    DRY_RUN=true; shift ;;
    --backup)     BACKUP=true; shift ;;
    --root)       GAME_ROOT="$2"; shift 2 ;;
    --stop-timeout) STOP_TIMEOUT="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,18p' "$0"
      exit 0 ;;
    *) echo "unknown flag: $1"; exit 1 ;;
  esac
done

if [[ -z "$SERVER_ID" ]]; then
  echo "error: --server-id required (or set EXPRESS233_SERVER_ID)"
  exit 1
fi

# ═══════════════ 路径定义 ═══════════════
FINAL_DIR="${GAME_ROOT}/${SERVER_ID}"
TMP_DIR="${GAME_ROOT}/.tmp/${SERVER_ID}"
LOG_DIR="${FINAL_DIR}/logs"
RUN_DIR="${FINAL_DIR}/run"
PID_FILE="${RUN_DIR}/server.pid"
BACKUP_DIR="${GAME_ROOT}/.backup/${SERVER_ID}"

log() { echo "[$(date '+%H:%M:%S')] [${SERVER_ID}] $*"; }

# ═══════════════ Step 1: 拉取到临时目录 ═══════════════
log "Step 1/5: pulling to staging area..."
rm -rf "$TMP_DIR"
mkdir -p "$TMP_DIR"

if $DRY_RUN; then
  log "[dry-run] would pull to $TMP_DIR"
else
  $EXPRESS233_BIN pull \
    --server-id "$SERVER_ID" \
    --dest "$TMP_DIR" \
    --skip-hook \
    "${VERSION_ARGS[@]}"
  log "pull complete: $(find "$TMP_DIR" -type f | wc -l) files"
fi

# ═══════════════ Step 2: 停止旧服务 ═══════════════
stop_server() {
  if [[ ! -f "$PID_FILE" ]]; then
    log "Step 2/5: no PID file, skip stop"
    return
  fi
  local pid
  pid=$(cat "$PID_FILE")
  if ! kill -0 "$pid" 2>/dev/null; then
    log "Step 2/5: PID $pid not running, skip stop"
    rm -f "$PID_FILE"
    return
  fi

  log "Step 2/5: stopping server PID=$pid..."
  if $DRY_RUN; then
    log "[dry-run] would kill $pid"
    return
  fi

  kill "$pid" 2>/dev/null || true
  local waited=0
  while kill -0 "$pid" 2>/dev/null && [[ $waited -lt $STOP_TIMEOUT ]]; do
    sleep 1
    waited=$((waited + 1))
  done

  if kill -0 "$pid" 2>/dev/null; then
    log "WARNING: server did not stop after ${STOP_TIMEOUT}s, sending SIGKILL"
    kill -9 "$pid" 2>/dev/null || true
    sleep 1
  fi
  rm -f "$PID_FILE"
  log "server stopped"
}

if $DRY_RUN; then
  log "Step 2/5: [dry-run] would stop server"
else
  stop_server
fi

# ═══════════════ Step 3: 备份（可选）═══════════════════
if $BACKUP && [[ -d "$FINAL_DIR" ]] && ! $DRY_RUN; then
  mkdir -p "$BACKUP_DIR"
  ts=$(date '+%Y%m%d_%H%M%S')
  archive="${BACKUP_DIR}/${SERVER_ID}_${ts}.tar.gz"
  log "Step 3/5: backing up to $archive"
  tar -czf "$archive" \
    --exclude='logs' \
    --exclude='run' \
    -C "$(dirname "$FINAL_DIR")" \
    "$(basename "$FINAL_DIR")"
else
  log "Step 3/5: backup skipped ($BACKUP or no existing dir)"
fi

# ═══════════════ Step 4: 替换文件 ═══════════════
log "Step 4/5: swapping files..."
mkdir -p "$LOG_DIR" "$RUN_DIR"

if $DRY_RUN; then
  log "[dry-run] would sync $TMP_DIR → $FINAL_DIR"
else
  # 用 rsync 精确同步，保留 logs/ 和 run/ 不动
  if command -v rsync &>/dev/null; then
    rsync -a --delete \
      --exclude='logs/' \
      --exclude='run/' \
      "$TMP_DIR/" "$FINAL_DIR/"
  else
    # 无 rsync 回退: 手动删除旧文件（保留 logs/ run/），再复制新文件
    find "$FINAL_DIR" -mindepth 1 -maxdepth 1 \
      ! -name 'logs' ! -name 'run' \
      -exec rm -rf {} +
    cp -a "$TMP_DIR/"* "$FINAL_DIR/" 2>/dev/null || true
    cp -a "$TMP_DIR/".[!.]* "$FINAL_DIR/" 2>/dev/null || true
  fi
  log "files synced (logs/ and run/ preserved)"
fi

# ═══════════════ Step 5: 启动新服务 ═══════════════
log "Step 5/5: starting server..."
if $DRY_RUN; then
  log "[dry-run] would run post_hook"
else
  # 执行 post_hook (如果存在)
  HOOK_SCRIPT=""
  if [[ -f "$FINAL_DIR/scripts/restart.sh" ]]; then
    HOOK_SCRIPT="$FINAL_DIR/scripts/restart.sh"
  fi

  if [[ -n "$HOOK_SCRIPT" ]]; then
    chmod +x "$HOOK_SCRIPT"
    env SERVER_ID="$SERVER_ID" EXPRESS233_SERVER_ID="$SERVER_ID" "$HOOK_SCRIPT"
    log "post_hook executed"
  else
    log "no restart script found at scripts/restart.sh — start manually"
  fi
fi

# ═══════════════ 清理 ═══════════════
if ! $DRY_RUN; then
  rm -rf "$TMP_DIR"
  log "staging cleaned"
fi

log "deploy complete!"
