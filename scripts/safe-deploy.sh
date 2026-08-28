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
#   数据目录:  GAME_ROOT/{server_id}/data/     (部署不触碰，持久保留)
#   日志目录:  GAME_ROOT/{server_id}/logs/     (部署不触碰，持久保留)
#   PID 文件:  GAME_ROOT/{server_id}/run/server.pid
#   备份目录:  GAME_ROOT/.backup/{server_id}/  (--backup 开启，推送部署默认开启)
#
set -euo pipefail

# ═══════════════ 配置 ═══════════════
GAME_ROOT="${GAME_ROOT:-/opt/game-servers}"
EXPRESS233_BIN="${EXPRESS233_BIN:-express233-cli}"
STOP_TIMEOUT="${STOP_TIMEOUT:-180}"       # 等待进程保存数据并正常退出的秒数
STOP_POLL_INTERVAL="${EXPRESS233_STOP_POLL_INTERVAL:-0.2}"
FORCE_KILL_AFTER_TIMEOUT="${EXPRESS233_FORCE_KILL_AFTER_TIMEOUT:-0}"
POST_KILL_SETTLE_SECONDS="${EXPRESS233_POST_KILL_SETTLE_SECONDS:-3}"
DEPLOY_LOCK_TIMEOUT="${EXPRESS233_DEPLOY_LOCK_TIMEOUT:-300}"
DRY_RUN=false
BACKUP=false
BACKUP_KEEP="${EXPRESS233_BACKUP_KEEP:-5}"
VERSION_ARGS=()
if [[ -n "${VERSION:-}" ]]; then
  VERSION_ARGS=(--version "$VERSION")
fi
TAG_ARGS=()
if [[ -n "${EXPRESS233_TAGS:-}" ]]; then
  IFS=',' read -r -a _express233_tags <<<"$EXPRESS233_TAGS"
  for _express233_tag in "${_express233_tags[@]}"; do
    [[ -n "$_express233_tag" ]] && TAG_ARGS+=(--tag "$_express233_tag")
  done
fi

# ═══════════════ 参数解析 ═══════════════
SERVER_ID="${EXPRESS233_SERVER_ID:-}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-id)  SERVER_ID="$2"; shift 2 ;;
    --version)    VERSION_ARGS=(--version "$2"); shift 2 ;;
    --dry-run)    DRY_RUN=true; shift ;;
    --backup)     BACKUP=true; shift ;;
    --root)       GAME_ROOT="$2"; shift 2 ;;
    --stop-timeout-seconds) STOP_TIMEOUT="$2"; shift 2 ;;
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
if [[ ! "$SERVER_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]]; then
  echo "error: invalid server_id (allowed: letters, digits, dot, underscore, hyphen)"
  exit 1
fi
if [[ ! "$STOP_TIMEOUT" =~ ^[1-9][0-9]*$ ]]; then
  echo "error: STOP_TIMEOUT must be a positive integer"
  exit 1
fi
if [[ ! "$BACKUP_KEEP" =~ ^[1-9][0-9]*$ ]]; then
  echo "error: EXPRESS233_BACKUP_KEEP must be a positive integer"
  exit 1
fi
if [[ ! "$DEPLOY_LOCK_TIMEOUT" =~ ^[0-9]+$ ]]; then
  echo "error: EXPRESS233_DEPLOY_LOCK_TIMEOUT must be a non-negative integer"
  exit 1
fi
if [[ "$FORCE_KILL_AFTER_TIMEOUT" != "0" && "$FORCE_KILL_AFTER_TIMEOUT" != "1" ]]; then
  echo "error: EXPRESS233_FORCE_KILL_AFTER_TIMEOUT must be 0 or 1"
  exit 1
fi
if [[ ! "$POST_KILL_SETTLE_SECONDS" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
  echo "error: EXPRESS233_POST_KILL_SETTLE_SECONDS must be a non-negative number"
  exit 1
fi

# ═══════════════ 路径定义 ═══════════════
FINAL_DIR="${GAME_ROOT}/${SERVER_ID}"
DEPLOY_RUN_ID="$(date '+%Y%m%d%H%M%S')-$$"
TMP_DIR="${GAME_ROOT}/.tmp/${SERVER_ID}/${DEPLOY_RUN_ID}"
LOG_DIR="${FINAL_DIR}/logs"
RUN_DIR="${FINAL_DIR}/run"
PID_FILE="${RUN_DIR}/server.pid"
BACKUP_DIR="${GAME_ROOT}/.backup/${SERVER_ID}"
LOCK_DIR="${GAME_ROOT}/.locks"
LOCK_FILE="${LOCK_DIR}/${SERVER_ID}.lock"
STATE_FILE="${FINAL_DIR}/.express233/deployment-state"
BACKUP_ARCHIVE=""
STAGED_DIGEST=""
DOWNTIME_STARTED=0
DEPLOY_SUCCEEDED=0
RECOVERY_ACTIVE=0

log() { echo "[$(date '+%H:%M:%S')] [${SERVER_ID}] $*"; }
cleanup() { rm -rf -- "$TMP_DIR"; }
trap cleanup EXIT

tree_digest() {
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$1" && find . -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | awk '{print $1}')
    return
  fi
  (cd "$1" && find . -type f -print0 | sort -z | xargs -0 shasum -a 256 | shasum -a 256 | awk '{print $1}')
}

pid_running() {
  local pid="$1" state
  kill -0 "$pid" 2>/dev/null || return 1
  state=$(ps -o stat= -p "$pid" 2>/dev/null | tr -d '[:space:]')
  [[ -n "$state" && "$state" != Z* ]]
}

current_release_matches() {
  [[ -f "$STATE_FILE" && -f "$PID_FILE" ]] || return 1
  [[ "$(sed -n '1p' "$STATE_FILE")" == "$STAGED_DIGEST" ]] || return 1
  local pid
  pid=$(cat "$PID_FILE")
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] && pid_running "$pid" || return 1
  if [[ -f "$FINAL_DIR/scripts/healthcheck.sh" ]]; then
    env SERVER_ID="$SERVER_ID" EXPRESS233_SERVER_ID="$SERVER_ID" "$FINAL_DIR/scripts/healthcheck.sh" 9>&- >/dev/null 2>&1 || return 1
  fi
  return 0
}

# ═══════════════ Step 1: 拉取到临时目录 ═══════════════
log "Step 1/5: pulling to staging area..."
mkdir -p "$TMP_DIR"

if $DRY_RUN; then
  log "[dry-run] would pull to $TMP_DIR"
else
  $EXPRESS233_BIN pull \
    --server-id "$SERVER_ID" \
    --dest "$TMP_DIR" \
    --skip-hook \
    --retries 1 \
    "${TAG_ARGS[@]}" \
    "${VERSION_ARGS[@]}"
  log "pull complete: $(find "$TMP_DIR" -type f | wc -l) files"
  STAGED_DIGEST=$(tree_digest "$TMP_DIR")
  log "staging verified: sha256=$STAGED_DIGEST"
fi

# 拉取和校验完成后才竞争目标锁；等待锁期间不会影响旧服务。
if ! $DRY_RUN; then
  command -v flock >/dev/null 2>&1 || { log "ERROR: flock is required"; exit 127; }
  mkdir -p "$LOCK_DIR"
  exec 9>"$LOCK_FILE"
  if ! flock -w "$DEPLOY_LOCK_TIMEOUT" 9; then
    log "ERROR: target is locked by another deployment"
    exit 75
  fi
  log "target lock acquired"
  if current_release_matches; then
    log "same resolved release is already healthy; idempotent no-op"
    exit 0
  fi

  # Artifact 内配置是当前中央配置源，冲突时以新文件为准；旧实例额外维护的
  # 配置文件补入暂存目录。持久化 data/ 不进入 Artifact，换包时始终原地保留。
  if [[ -d "$FINAL_DIR/config" ]]; then
    mkdir -p "$TMP_DIR/config"
    cp -an "$FINAL_DIR/config/." "$TMP_DIR/config/"
    log "previous config scanned; missing local files migrated to staging"
  fi
fi

# ═══════════════ Step 3: 停止旧服务 ═══════════════
stop_server() {
  if [[ ! -f "$PID_FILE" ]]; then
    log "Step 3/5: no PID file, skip stop"
    return
  fi
  local pid
  pid=$(cat "$PID_FILE")
  if [[ ! "$pid" =~ ^[1-9][0-9]*$ ]]; then
    log "ERROR: invalid PID file: $PID_FILE"
    return 1
  fi
  if ! pid_running "$pid"; then
    log "Step 3/5: PID $pid not running, remove stale PID file"
    rm -f "$PID_FILE"
    return
  fi

  local executable_path
  executable_path=$(readlink -f "/proc/$pid/exe" 2>/dev/null || true)
  case "$executable_path" in
    "$FINAL_DIR"/*) ;;
    *)
      log "ERROR: refusing to stop unrelated PID=$pid executable=$executable_path"
      return 1
      ;;
  esac

  log "Step 3/5: sending SIGTERM to PID=$pid; waiting for graceful exit..."
  if $DRY_RUN; then
    log "[dry-run] would kill $pid"
    return
  fi

  if ! kill -TERM "$pid" 2>/dev/null; then
    log "ERROR: failed to send SIGTERM to PID=$pid"
    return 1
  fi
  local started=$SECONDS forced=0
  while pid_running "$pid" && (( SECONDS - started < STOP_TIMEOUT )); do
    sleep "$STOP_POLL_INTERVAL"
  done

  if pid_running "$pid"; then
    if [[ "$FORCE_KILL_AFTER_TIMEOUT" != "1" ]]; then
      log "ERROR: server did not exit normally after ${STOP_TIMEOUT}s; deployment aborted without swapping files"
      return 1
    fi
    log "WARNING: graceful stop timed out; force-kill policy enabled, sending SIGKILL"
    forced=1
    kill -KILL "$pid" 2>/dev/null || return 1
    while pid_running "$pid"; do sleep "$STOP_POLL_INTERVAL"; done
    if [[ "$POST_KILL_SETTLE_SECONDS" != "0" ]]; then
      log "waiting ${POST_KILL_SETTLE_SECONDS}s for database/network leases to release"
      sleep "$POST_KILL_SETTLE_SECONDS"
    fi
  fi
  rm -f "$PID_FILE"
  if [[ "$forced" == "1" ]]; then
    log "server was force-killed after $((SECONDS - started))s"
  else
    log "server exited normally after $((SECONDS - started))s; MySQL flush completed by the server shutdown path"
  fi
}

# ═══════════════ Step 2: 停服前备份（不占用停机窗口）═══════════════════
if $BACKUP && [[ -d "$FINAL_DIR" ]] && ! $DRY_RUN; then
  mkdir -p "$BACKUP_DIR"
  ts=$(date '+%Y%m%d_%H%M%S')_$$
  BACKUP_ARCHIVE="${BACKUP_DIR}/${SERVER_ID}_${ts}.tar.gz"
  log "Step 2/5: preparing rollback archive before downtime: $BACKUP_ARCHIVE"
  tar -czf "$BACKUP_ARCHIVE" \
    --exclude='logs' \
    --exclude='run' \
    -C "$(dirname "$FINAL_DIR")" \
    "$(basename "$FINAL_DIR")"
  tar -tzf "$BACKUP_ARCHIVE" >/dev/null
  mapfile -t old_backups < <(
    find "$BACKUP_DIR" -maxdepth 1 -type f -name "${SERVER_ID}_*.tar.gz" -printf '%T@ %p\n' |
      sort -nr | tail -n "+$((BACKUP_KEEP + 1))" | cut -d' ' -f2-
  )
  for old_backup in "${old_backups[@]}"; do
    rm -f -- "$old_backup"
  done
else
  log "Step 2/5: backup skipped ($BACKUP or no existing dir)"
fi

# ═══════════════ Step 4: 替换文件 ═══════════════
sync_release_tree() {
  local source_dir="$1"
  local target_dir="$2"
  mkdir -p "$target_dir/data" "$target_dir/logs" "$target_dir/run"
  if command -v rsync &>/dev/null; then
    # 仅保留发布根目录的持久化目录。未锚定的规则会错误保留历史包装目录
    # 内的 data/logs/run，导致 rsync 无法删除废弃目录并阻断后续发布。
    rsync -a --delete \
      --exclude='/data/***' \
      --exclude='/logs/***' \
      --exclude='/run/***' \
      --exclude='/.env' \
      "$source_dir/" "$target_dir/"
  else
    find "$target_dir" -mindepth 1 -maxdepth 1 \
      ! -name 'data' ! -name 'logs' ! -name 'run' ! -name '.env' \
      -exec rm -rf {} +
    cp -a "$source_dir/"* "$target_dir/" 2>/dev/null || true
    cp -a "$source_dir/".[!.]* "$target_dir/" 2>/dev/null || true
  fi
}

rollback_release() {
  if [[ -z "$BACKUP_ARCHIVE" || ! -f "$BACKUP_ARCHIVE" ]]; then
    log "ROLLBACK unavailable: no backup archive"
    return 1
  fi
  log "ROLLBACK: stopping failed release"
  if ! stop_server; then
    log "ROLLBACK failed: failed release did not stop safely"
    return 1
  fi
  local rollback_tmp
  rollback_tmp=$(mktemp -d "${GAME_ROOT}/.rollback-${SERVER_ID}-XXXXXX")
  if ! tar -xzf "$BACKUP_ARCHIVE" -C "$rollback_tmp"; then
    rm -rf "$rollback_tmp"
    return 1
  fi
  log "ROLLBACK: restoring $BACKUP_ARCHIVE"
  sync_release_tree "$rollback_tmp/$SERVER_ID" "$FINAL_DIR"
  rm -rf "$rollback_tmp"
  if [[ ! -f "$FINAL_DIR/scripts/restart.sh" ]]; then
    log "ROLLBACK failed: previous restart script missing"
    return 1
  fi
  chmod +x "$FINAL_DIR/scripts/restart.sh"
  env SERVER_ID="$SERVER_ID" EXPRESS233_SERVER_ID="$SERVER_ID" "$FINAL_DIR/scripts/restart.sh" 9>&-
  log "ROLLBACK complete"
}

fail_and_rollback() {
  local reason="$1"
  log "ERROR: $reason"
  if rollback_release; then
    rm -rf "$TMP_DIR"
    log "deployment failed; previous release restored"
  else
    log "CRITICAL: deployment and rollback both failed"
  fi
  exit 1
}

recover_interrupted_deployment() {
  local signal_name="$1"
  trap - HUP INT TERM
  if [[ "$DOWNTIME_STARTED" == "1" && "$DEPLOY_SUCCEEDED" != "1" && "$RECOVERY_ACTIVE" != "1" ]]; then
    RECOVERY_ACTIVE=1
    log "WARNING: deployment interrupted by $signal_name; restoring previous release"
    rollback_release || log "CRITICAL: interrupted deployment could not restore the previous release"
  fi
  exit 130
}
trap 'recover_interrupted_deployment SIGHUP' HUP
trap 'recover_interrupted_deployment SIGINT' INT
trap 'recover_interrupted_deployment SIGTERM' TERM

if $DRY_RUN; then
  log "Step 3/5: [dry-run] would send SIGTERM and wait for normal exit"
else
  DOWNTIME_STARTED=1
  if ! stop_server; then
    DOWNTIME_STARTED=0
    log "deployment stopped before file swap; existing release remains untouched"
    exit 1
  fi
fi

log "Step 4/5: swapping files..."
mkdir -p "$LOG_DIR" "$RUN_DIR"

if $DRY_RUN; then
  log "[dry-run] would sync $TMP_DIR → $FINAL_DIR"
else
  sync_release_tree "$TMP_DIR" "$FINAL_DIR"
  log "files synced (data/, logs/, run/ and .env preserved)"
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
    if ! env SERVER_ID="$SERVER_ID" EXPRESS233_SERVER_ID="$SERVER_ID" "$HOOK_SCRIPT" 9>&-; then
      fail_and_rollback "new release failed to start"
    fi
    log "post_hook executed"
  else
    fail_and_rollback "restart script missing at scripts/restart.sh"
  fi
  if [[ -f "$FINAL_DIR/scripts/healthcheck.sh" ]]; then
    chmod +x "$FINAL_DIR/scripts/healthcheck.sh"
    if ! env SERVER_ID="$SERVER_ID" EXPRESS233_SERVER_ID="$SERVER_ID" "$FINAL_DIR/scripts/healthcheck.sh" 9>&-; then
      fail_and_rollback "new release health check failed"
    fi
    log "health check passed"
  fi
  mkdir -p "$(dirname "$STATE_FILE")"
  state_tmp="${STATE_FILE}.tmp.$$"
  printf '%s\n%s\n%s\n%s\n%s\n%s\n' "$STAGED_DIGEST" "${EXPRESS233_PROJECT:-}" "${VERSION:-latest}" "$SERVER_ID" "${EXPRESS233_DEPLOYMENT_ID:-}" "${EXPRESS233_IDEMPOTENCY_KEY:-}" >"$state_tmp"
  mv -f "$state_tmp" "$STATE_FILE"
  DEPLOY_SUCCEEDED=1
  log "release state committed"
fi

# ═══════════════ 清理 ═══════════════
if ! $DRY_RUN; then
  cleanup
  trap - EXIT
  log "staging cleaned"
fi

log "deploy complete!"
