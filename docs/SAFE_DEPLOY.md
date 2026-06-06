# 游戏逻辑服安全部署方案

## 场景

一台物理/云服务器上运行多个游戏逻辑服实例，每个实例由 `server_id` 标识（如 `game-logic-01`, `game-logic-02`, `game-logic-03`）。当中央控制台发布新版本后，需要通过 SSH 远程执行安全部署——不能直接覆盖正在运行的二进制和配置文件。

## 目录布局

所有路径以 `GAME_ROOT`（默认 `/opt/game-servers`）为根：

```
/opt/game-servers/
├── game-logic-01/              ← 最终运行目录
│   ├── bin/game-server.sh      # 启动脚本
│   ├── scripts/restart.sh      # post_hook 重启脚本
│   ├── game.properties         # 已替换的配置文件
│   ├── application.yaml        # 已替换的配置文件
│   ├── run/server.pid          # PID 文件
│   └── logs/                   # 日志（部署不触碰，持久保留）
│       └── game-logic-01.log
├── game-logic-02/
│   └── ...                     # 同结构，不同配置值
├── game-logic-03/
│   └── ...
├── .tmp/                       ← 临时暂存区（部署后自动清理）
│   ├── game-logic-01/          # 每个 server_id 独立临时目录
│   ├── game-logic-02/
│   └── game-logic-03/
└── .backup/                    ← 可选备份（--backup 开启）
    ├── game-logic-01/
    └── ...
```

**关键设计**：

- 每个 `server_id` 拥有独立的临时目录，支持并行或串行部署
- `logs/` 和 `run/` 在部署过程中不被删除，日志持久保留
- `.tmp/` 部署完成后自动清理，不占磁盘

## 单实例部署流程

```
┌─────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  1. Pull    │───▶│  2. Stop     │───▶│  3. Backup   │───▶│  4. Swap     │───▶│  5. Start    │
│  to .tmp/   │    │  old server  │    │  (optional)  │    │  files       │    │  new server  │
└─────────────┘    └──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘
```

**Step 1 — 拉取到临时目录**

```bash
express233 pull --server-id game-logic-01 --dest /opt/game-servers/.tmp/game-logic-01 --skip-hook
```

拉到 `.tmp/` 而不是最终目录，确保不影响正在运行的服务。

**Step 2 — 停止旧服务**

```bash
PID=$(cat /opt/game-servers/game-logic-01/run/server.pid)
kill $PID
# 等待退出（超时 10s）
for i in $(seq 1 10); do kill -0 $PID 2>/dev/null || break; sleep 1; done
# 仍未退出则 SIGKILL
kill -0 $PID 2>/dev/null && kill -9 $PID
```

**Step 3 — 备份（可选）**

```bash
tar -czf /opt/game-servers/.backup/game-logic-01/$(date +%Y%m%d_%H%M%S).tar.gz \
  --exclude=logs --exclude=run \
  -C /opt/game-servers game-logic-01
```

**Step 4 — 替换文件**

```bash
rsync -a --delete \
  --exclude='logs/' --exclude='run/' \
  /opt/game-servers/.tmp/game-logic-01/ \
  /opt/game-servers/game-logic-01/
```

`rsync --exclude` 确保 `logs/` 和 `run/` 不被覆盖。无 rsync 时回退为手动删旧+拷贝。

**Step 5 — 启动新服务**

```bash
SERVER_ID=game-logic-01 /opt/game-servers/game-logic-01/scripts/restart.sh
```

post_hook 脚本负责启动进程并写入 PID 文件。

## 一键部署脚本

### 单实例

```bash
# SSH 到目标机器后执行:
export EXPRESS233_SERVER=http://central:23380
export EXPRESS233_TOKEN=your_pull_token

bash scripts/safe-deploy.sh --server-id game-logic-01
bash scripts/safe-deploy.sh --server-id game-logic-01 --version 1.0.0  # 指定版本
bash scripts/safe-deploy.sh --server-id game-logic-01 --backup          # 带备份
bash scripts/safe-deploy.sh --server-id game-logic-01 --dry-run         # 预览不执行
```

### 批量（同机多服）

```bash
# 串行部署 3 个实例，单个失败不阻断其余:
bash scripts/batch-deploy.sh \
  --servers "game-logic-01,game-logic-02,game-logic-03"

# 从文件读取:
bash scripts/batch-deploy.sh --file servers.txt

# 指定版本:
bash scripts/batch-deploy.sh --servers "game-logic-01,game-logic-02" --version 1.2.0
```

## SSH 远程一行命令

```bash
# 从本地 SSH 到远程机器执行部署:
ssh deploy@game-host \
  'EXPRESS233_SERVER=http://central:23380 EXPRESS233_TOKEN=xxx \
   bash /opt/game-servers/scripts/safe-deploy.sh \
   --server-id game-logic-01'

# 批量部署:
ssh deploy@game-host \
  'EXPRESS233_SERVER=http://central:23380 EXPRESS233_TOKEN=xxx \
   bash /opt/game-servers/scripts/batch-deploy.sh \
   --servers "game-logic-01,game-logic-02,game-logic-03"'
```

## 配置模板替换验证

server.yaml 中每个 server_id 的配置在拉取时自动替换。以下示例展示同一份 `game.properties` 在不同 server_id 下的替换结果：

**模板（上传的原始文件）**：
```properties
server.id=${server.id}
server.port=${server.port}
db.host=${db.host}
```

**game-logic-01 拉取后**：
```properties
server.id=game-logic-01
server.port=9001
db.host=db-ecs-a.internal
```

**game-logic-02 拉取后**：
```properties
server.id=game-logic-02
server.port=9002
db.host=db-ecs-b.internal
```

部署前可在 Web UI「部署」Tab 或 CLI `express233 preview` 预览替换结果：

```bash
express233 preview --server-id game-logic-01
express233 preview --server-id game-logic-02
```

## 日志管理

每个 game-server 实例的日志输出到独立目录：

```
/opt/game-servers/game-logic-01/logs/game-logic-01.log
/opt/game-servers/game-logic-02/logs/game-logic-02.log
/opt/game-servers/game-logic-03/logs/game-logic-03.log
```

部署过程中 `logs/` 目录不会被删除或覆盖，日志文件持续追加。如需日志轮转，在 restart.sh 中集成 logrotate 或使用 nohup + 日期文件名。

## 故障恢复

```bash
# 回滚到上一发布版本:
express233 rollback --server-id game-logic-01 --dest /opt/game-servers/.tmp/game-logic-01
# 然后手动执行 safe-deploy 的 Step 2-5

# 从备份恢复:
tar -xzf /opt/game-servers/.backup/game-logic-01/20260605_120000.tar.gz \
  -C /opt/game-servers/
bash /opt/game-servers/game-logic-01/scripts/restart.sh
```

## 注意事项

1. **永远不要直接覆盖运行中的二进制**——Linux 下虽然技术上可行（text segment 已加载到内存），但配置文件变更可能导致运行时异常
2. **stop → swap → start 的串行顺序不可变**——确保旧进程完全退出后再替换文件
3. **`--skip-hook` 在 safe-deploy 中默认开启**——因为 stop/start 由脚本自己管理，post_hook 只负责启动
4. **批量部署默认串行**——避免同时停多个服导致玩家全掉线；如需并行加 `--parallel`
5. **部署脚本幂等**——重复执行安全，已停止的服务不会报错
