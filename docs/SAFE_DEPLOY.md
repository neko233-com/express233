# 游戏逻辑服安全部署

运行中的游戏服使用 `scripts/safe-deploy.sh` 执行 `stage → lock → stop → swap → start`。`express233-cli pull` 与 `deploy` 是同一拉取实现，直接解压到 `--dest` 并默认执行 Hook，本身没有安全停服和换包步骤。本文不用于更新中央控制台；控制台更新见 [DEPLOY.md](DEPLOY.md)。

## 前置条件与实例隔离

目标为 Linux，安装 Bash、`express233-cli`、`flock`、GNU tar/coreutils 和 **rsync**。使用 `/proc/{pid}/exe` 校验进程归属。缺少 rsync、restart.sh 或 healthcheck.sh 时在停服前拒绝执行，不再使用忽略复制错误的回退分支。控制器要求节点 `safe-deploy.sh --capabilities` 返回 `express233.safe-deploy/v2`。

每个 `server_id` 必须有独立运行目录，且在同一 `GAME_ROOT` 下唯一；不同项目不能共用同一个实例目录。默认根目录 `/opt/game-servers`：

```text
GAME_ROOT/
├── game-logic-01/
│   ├── scripts/restart.sh       # 必需：启动并写 PID
│   ├── scripts/healthcheck.sh   # 必需：业务就绪检查
│   ├── config/                 # 新包同名文件优先，补入旧目录独有文件
│   ├── data/                   # 原地保留
│   ├── logs/                   # 原地保留
│   ├── run/server.pid          # 实例 PID
│   └── .env                    # 原地保留
├── .tmp/game-logic-01/{run_id}/ # 每次独立暂存
├── .locks/game-logic-01.lock   # 同实例独占锁
└── .backup/game-logic-01/      # 仅 --backup 时创建
```

运行数据不得放入发布包，凭据保留在受保护的运行环境或 `.env` 中，不写入 Git。运行目录中的其它文件属于发布内容，换包时可能删除；不要把业务持久文件放在未受保护的位置。

## 一次发布的实际顺序

1. **Stage**：拉取到 `.tmp/{server_id}/{run_id}/`，使用 `--skip-hook --retries 1`，完成解压后计算内容 SHA-256。此时不影响旧服务；内容指纹用于幂等比较，不等同于官方 Release 的 `SHA256SUMS` 校验。
2. **锁与幂等**：获得 `.locks/{server_id}.lock` 后再检查已记录指纹、PID 是否存活，以及可选健康脚本。命中则退出，不重启。锁默认最多等待 300 秒，超时退出码 `75`。
3. **配置与可选备份**：旧 `config/` 中新包没有的文件补入暂存；同名冲突以新包为准。开启 `--backup` 时，在持锁后、停服前创建发布回滚归档，仅锚定排除实例根目录的 `data/`、`logs/` 和 `run/`，保留最近 5 份。`config/`、`.env` 和包内嵌套的 `assets/data/`、`assets/logs/` 等发布内容仍归档；归档创建或校验非零会中止发布，不忽略 tar 错误。
4. **Stop**：读取并验证 PID，确认 `/proc/{pid}/exe` 位于该实例目录才发送 SIGTERM。默认每 200ms 检查一次，最多等 180 秒；进程未退出就中止，不换包。
5. **Swap**：仅在确认退出后执行 rsync，根目录的 `data/`、`logs/`、`run/` 与 `.env` 原地保留；其它发布文件同步并删除旧的多余文件。
6. **Start 与验收**：执行 `scripts/restart.sh`，如存在 `scripts/healthcheck.sh` 则最多等待 30 秒就绪，最后原子写入部署状态并清理本次暂存目录。

游戏服必须自己处理 SIGTERM、保存玩家数据、完成数据库 Flush、关闭组件并正常退出；脚本只能确认退出，不能证明业务已成功保存。正常发布禁止 SIGKILL；只有人工明确授权的应急处置才可使用 `EXPRESS233_FORCE_KILL_AFTER_TIMEOUT=1`。如实例受 systemd 或其它守护程序管理，还须由游戏服脚本协调自动重启和停止策略，避免守护程序重新拉起旧进程。

部署 post-hook 返回 75 表示无法确认运行实例已安全停止，外层保留现场和已有回滚归档，不调用旧版本钩子；其他非零状态沿用回滚流程。仅在 post-hook 和目标机 safe-deploy.sh 均支持此约定后，该保护才在线上生效。

`healthcheck.sh` 必须由应用核验固定版本、服号与业务就绪；通用部署器不能推断某个应用的真实客户端握手。幂等状态中的 PID 存活检查不能替代业务健康检查。

## 单实例命令

由运行环境注入中央地址、项目和凭据；不要把 Token、密码或真实 SSH 信息拼进命令参数。CLI 的 Token 模式目前使用查询参数，代理与客户端错误日志也必须脱敏。

```bash
: "${EXPRESS233_SERVER:?inject the central URL}"
: "${EXPRESS233_PROJECT:?set the project}"
# 凭据：EXPRESS233_TOKEN，或 EXPRESS233_USERNAME / EXPRESS233_PASSWORD。
bash scripts/safe-deploy.sh --server-id game-logic-01 --dry-run
bash scripts/safe-deploy.sh --server-id game-logic-01 --version 1.2.0 --backup
```

`--dry-run` 不下载、不加锁、不停服、不换包、不运行 Hook，但会创建临时目录及实例的 `logs/`、`run/` 目录，再清理本次暂存目录。它不是完整健康检查或完全无文件系统副作用的模拟。

| 配置 | 默认值 / 含义 |
|---|---|
| `--root` / `GAME_ROOT` | `/opt/game-servers` |
| `--stop-timeout-seconds` / `STOP_TIMEOUT` | `180` 秒；旧别名 `--stop-timeout` 仍可用 |
| `EXPRESS233_DEPLOY_LOCK_TIMEOUT` | `300` 秒 |
| `EXPRESS233_HEALTH_CHECK_TIMEOUT_SECONDS` | `30` 秒 |
| `EXPRESS233_HEALTH_CHECK_POLL_INTERVAL_SECONDS` | `1` 秒 |
| `EXPRESS233_BACKUP_KEEP` | `5` 份，仅开启备份时生效 |
| `EXPRESS233_BIN` | `express233-cli` |

手工调用默认不备份；控制台 SSH 推送默认传 `--backup`，绑定可选择“跳过旧版备份”。安全脚本再向 `restart.sh` 传入对应的 `EXPRESS233_SKIP_BACKUP=true/false`，仓库自己的启动脚本也应遵守该策略。

该归档仅用于发布文件回滚，**不备份实例根目录的运行数据或数据库**。旧服务尚在运行时读取 SQLite 主库、WAL/SHM 等活动文件既不保证一致性，也可能因文件变化使 tar 返回非零，因此根 `data/` 在归档和恢复两侧均被排除。日志中的 release rollback archive 不代表数据库备份成功；数据库恢复须依赖另行验证的停服快照或数据库自身的一致性备份流程。

排除模式锚定到 `${server_id}/data`、`${server_id}/logs`、`${server_id}/run`，与发布替换时只保护根目录的语义一致；以前裸 `logs/run` 会连带遗漏包内嵌套同名目录，现在这些目录作为发布内容保留在归档中。自动回滚仍用同样的 rsync 保留实时根 `data/`、`logs/`、`run/`、`.env`，只恢复旧发布文件；`config/` 属于被恢复的发布内容。`.env` 虽包含在归档中，自动回滚不会覆盖当前运行环境，归档及其父目录须限制访问。

Metrics233 私有部署 Hook 的完整停服数据/identity/key 快照是独立门禁，本次发布归档排除规则不替代、不跳过也不修改该快照流程。其它应用同样须自行建立和验收数据库备份，不能只凭 `--backup` 认定数据库可恢复。

## 批量与 SSH

```bash
bash scripts/batch-deploy.sh --servers 'game-logic-01,game-logic-02' --version 1.2.0
bash scripts/batch-deploy.sh --file servers.txt
```

当前批量脚本只有串行模式，首个失败立即停止，后续目标保持不动。支持 `--servers`、`--file`、`--version`、`--root`；**没有** `--parallel`、`--backup` 或 `--dry-run`。它调用单实例脚本时默认不备份；需要逐目标备份策略时使用控制台绑定，或明确逐个调用单实例脚本。

SSH 手工示例使用未跟踪的本机 SSH alias；远端环境文件由运维预先配置并限制访问，不把真实连接信息或 Secret 放入仓库：

```bash
ssh game-node sh -s <<'REMOTE'
set -eu
set -a
. /etc/express233/node.env
set +a
bash /opt/express233/scripts/safe-deploy.sh --server-id game-logic-01 --version 1.2.0 --backup
REMOTE
```

控制台推送也经标准输入发送环境与脚本，详见 [SSH_PUSH_DEPLOY.md](SSH_PUSH_DEPLOY.md)。部署前在项目“预览”检查最终配置；预览中的配置正文同样可能敏感，不直接复制到公开日志或截图。

## 失败与恢复

- 下载、锁等待或安全停服失败发生在换包前，旧发布文件不被替换。收到 SIGTERM 的旧进程可能仍在退出，需要核查实际状态。
- 启动失败或健康检查失败且应用允许恢复时，有备份才尝试安全停止失败进程、恢复旧发布文件并重新启动；跳过备份时无自动回滚源。Hook 返回 75 或收到 HUP/INT/TERM 时保留现场，不重放 Hook。
- `run/deployment-attempt` 原子记录阶段、run_id、项目和版本。中断或阻断后的下一次部署在触碰服务前返回 75；运维先核验进程、刷库和版本，明确选择 `--acknowledge-recovery <原 run_id>` 后才能发起新尝试。此参数仅确认人工已完成核验，不自动证明业务恢复。
- 正常退出路径会清理本次暂存目录；断电或无法捕获的终止可能留下临时文件。
- 需要回到某个仍可拉取的旧版本时，重新运行 `safe-deploy.sh --version ...`。不要直接 `express233-cli rollback --dest 运行目录`，也不要向运行目录解压备份。
- 人工恢复归档必须先核对目标、停止正确实例并确认退出，再按同样的保留规则恢复发布文件；不得顺带回退实时玩家数据。

更具体的进程归属、退出与业务健康约定见 [DEPLOYMENT_SCRIPT_CONTRACT.md](DEPLOYMENT_SCRIPT_CONTRACT.md)。
