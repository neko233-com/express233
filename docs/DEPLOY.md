# 部署指南

`express233-server` 是保存项目、版本、配置和发布历史的中央控制台；游戏服节点由 `express233-cli` 拉取，再由安全部署脚本管理进程。更新中央控制台与发布游戏服是两项独立操作。

## 中央控制台部署

### Docker Compose

```bash
docker compose up -d --build
```

仓库 Compose 从当前源码构建，持久卷 `express233-data` 挂载到 `/data`。更新时复用原卷与数据路径，不执行 `docker compose down -v`。生产入口配置 HTTPS；密码、加密密钥等由运行环境的 Secret 注入，不写入 Compose 或 Git。

### Kubernetes（Helm）

先选择已经发布的镜像版本，通过环境变量 `EXPRESS233_IMAGE_TAG` 指定；镜像 tag 不带 Git tag 的 `v` 前缀。

```bash
: "${EXPRESS233_IMAGE_TAG:?set an existing image tag}"
helm upgrade --install express233 ./deploy/helm/express233-server \
  --namespace express233 --create-namespace \
  --set-string "image.tag=$EXPRESS233_IMAGE_TAG" \
  --set ingress.enabled=true \
  --set 'ingress.hosts[0].host=express233.example.invalid'
```

默认单副本，PVC 挂载 `/data`；其中保存 SQLite、租户配置与版本文件。保留现有 PVC，不通过更换数据路径“重建”已有控制台。当前 chart 的 `env` 模板只接入 `EXPRESS233_DATA`；需要其它 Secret 时，在受控部署清单中添加 `secretKeyRef`，不要假设任意 `--set env.*` 都会生效。

### 二进制安装与更新

从官方 Release 获取对应平台的 `express233-server-{os}-{arch}`、`SHA256SUMS` 和 `PROVENANCE.json`，先在独立目录校验摘要、版本和 commit。下载与校验示例见 [GitHub Actions](GITHUB_ACTIONS.md)。安装脚本和内置 `update` 命令不能替代这套验收：当前安装脚本不校验官方摘要，内置更新返回 `scheduled` 也不表示新进程已就绪。

已有服务按以下顺序更新：

1. 确认当前可执行文件、服务单元、PID、启动参数、数据目录与 Secret 来源；只替换控制台二进制。
2. 下载并校验新包；是否保留旧二进制备份按本次运维策略决定，不自动创建数据库回退点。
3. 校验 PID 身份后发送 SIGTERM，等待旧进程完全退出。使用 systemd 时先处理自动重启和停止超时策略，防止自动拉起旧进程或超时 SIGKILL；超时中止，不换包。
4. 替换二进制并沿用原启动参数、数据目录和配置。禁止覆盖运行中的文件、清空数据、重新初始化账号或替换既有密钥。
5. 检查进程身份、`version` 的版本与 commit、`/healthz`、`/readyz` 和实际页面；只读比较原项目、版本、配置与部署历史。

无数据库迁移的二进制回滚只恢复旧程序，保留当前数据库和配置；数据库恢复应单独决策，不能覆盖新服务已经接受的用户写入。此流程不操作游戏服进程。

## 游戏服节点

凭据由 Secret 或本机未跟踪配置注入 `EXPRESS233_TOKEN`，或注入 `EXPRESS233_USERNAME` / `EXPRESS233_PASSWORD`。不要使用含凭据的 CLI 参数，也不要在日志中运行 `config show`；当前命令会原样输出配置中的密码和 Token。

```bash
: "${EXPRESS233_SERVER:?inject the central URL}"
: "${EXPRESS233_PROJECT:?set the project}"
# 仅拉取到独立暂存目录，不操作运行中的实例。
STAGING_DIR=$(mktemp -d)
express233-cli pull --server-id game-logic-01 --dest "$STAGING_DIR" --skip-hook
```

`express233-cli deploy` 是 `pull` 的别名：下载完成后直接解压到 `--dest`，默认执行包内 Hook；它不会自行完成停服、换包或回滚。`rollback` 也调用该拉取流程，不能直接用于覆盖运行目录。

运行中的 Linux 节点使用 [safe-deploy.sh](SAFE_DEPLOY.md)：

```bash
bash scripts/safe-deploy.sh --server-id game-logic-01 --dry-run
bash scripts/safe-deploy.sh --server-id game-logic-01 --version 1.2.0 --backup
```

第二条会真实发布；目标版本必须已发布。`--backup` 可按策略省略，省略后没有自动回滚归档。多实例、数据保留与 SSH 推送规则见 [安全部署](SAFE_DEPLOY.md) 和 [SSH 推送](SSH_PUSH_DEPLOY.md)。

## 监控、备份与恢复

- 健康：`GET /healthz`；就绪：`GET /readyz`；指标：`GET /metrics`。Helm 的 `serviceMonitor.enabled` 需要 Prometheus Operator。
- API：`/docs/` 与 `/api/openapi.yaml`；审计：账号与管理中的审计入口或 `GET /api/audit-logs`。
- `scripts/backup-data.sh` 只是数据目录 tar 归档，不调用 SQLite 在线备份接口；先停止中央服务并确认退出，再备份，或使用另外经过验证的一致性备份方案。
- `scripts/restore-data.sh` 使用 `rsync --delete` 覆盖目标数据目录。只在明确授权的数据恢复中、停服并核对归档与目标后执行；它不属于普通升级步骤。

```bash
# 前置条件：中央服务已经正常退出，且已确认本次要备份/恢复的数据路径。
bash scripts/backup-data.sh /var/lib/express233 ./express233-backup
# 破坏性恢复示例：会删除目标目录中归档没有的文件。
bash scripts/restore-data.sh ./express233-backup.tar.gz /var/lib/express233
```

首次登录后在账号页面修改初始密码。日常拉取使用独立授权账号，Secret 不进入版本包、截图或日志。Cloudflare 入口适配见 [CLOUDFLARE.md](CLOUDFLARE.md)。