# express233

[![CI](https://github.com/neko233-com/express233/actions/workflows/ci.yml/badge.svg)](https://github.com/neko233-com/express233/actions/workflows/ci.yml)
[![Lint](https://github.com/neko233-com/express233/actions/workflows/lint.yml/badge.svg)](https://github.com/neko233-com/express233/actions/workflows/lint.yml)
[![CodeQL](https://github.com/neko233-com/express233/actions/workflows/codeql.yml/badge.svg)](https://github.com/neko233-com/express233/actions/workflows/codeql.yml)

游戏逻辑服 **拉模式** 部署：中央上传一次，SSH 集群 `express233-cli deploy` 拉齐；按 `server_id` 预览配置 diff。

## 工作流

| 步骤 | 说明 |
|------|------|
| 中央上传 | 项目 → 版本（草稿）→ 上传 / zip → **发布** |
| 配置约束 | 配置文件 **basename 全局唯一**；`server.yaml` 按 **文件名** 替换（无视路径） |
| 预览 | Web 或 `express233-cli preview` 查看每个键 before → after |
| 节点部署 | `express233-cli deploy` = 拉取 + 替换 + `post_hook` |

## 安装

**CLI（节点）**

```bash
curl -fsSL https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install.sh | bash
```

```powershell
iwr -useb https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install.ps1 | iex
```

**Server（中央）**

```bash
curl -fsSL https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install-server.sh | bash
```

```powershell
iwr -useb https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install-server.ps1 | iex
```

安装指定版本：

```bash
curl -fsSL https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install-server.sh | bash -s -- v0.1.0
```

```powershell
iwr -useb https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install-server.ps1 | iex; Install-Express233Server -Ver v0.1.0
```

或 `go install ./cmd/express233-cli ./cmd/express233-server`

Release 资产（tag `v*` 触发 [release.yml](.github/workflows/release.yml)）：

- `express233-cli-{linux,darwin,windows}-{amd64,arm64}[.exe]`
- `express233-server-{...}`（同上）
- 各文件 `.sha256` 校验

## 中央服运维命令

默认监听 `127.0.0.1:23380`，数据目录优先读取 `EXPRESS233_DATA`，否则使用 `~/.express233-server`。

首次安装后推荐顺序：

```bash
express233-server start
express233-server status
express233-server reset-root-password --password 'change-me-now'
```

常用命令：

```bash
express233-server port
express233-server set-port 32380
express233-server restart
express233-server enable-autostart
express233-server autostart-status
express233-server update
express233-server backup-config
express233-server reload-config
express233-server disable-autostart
express233-server restore-config
express233-server stop
```

说明：

- `start`：后台启动并写入 `run/server.pid`、`run/server-state.json`、`run/server.log`
- `enable-autostart` / `disable-autostart`：安装或移除原生开机自启动。Linux 使用 `systemd`，macOS 使用 `launchd`，Windows 使用 `schtasks /SC ONSTART`
- `autostart-status`：查看当前是否已安装原生开机自启动，以及当前是否 active
- `status`：查看当前 PID、访问地址、数据目录、默认端口
- `port`：查看默认端口 `23380` 与当前持久化监听地址
- `set-port`：修改中央服监听端口，默认会自动重启正在运行的中央服
- `update`：更新到最新 Release（或 `--version vX.Y.Z` 指定版本），自动替换二进制并重启中央服
- `reload-config`：校验并热重载租户 `server.yaml`，无需重启进程
- `backup-config` / `restore-config`：备份并恢复中央 `server.yaml`；`restore-config --default` 可恢复为示例模板
- `reset-root-password`：仅命令行强制重置 `root` 密码，适合忘记密码时救援
- 运行日志：默认 `info` 级别，写入 `run/server.log`，按大小滚动，避免单文件无限增长；高频 2xx 请求默认不记日志

PowerShell 示例：

```powershell
express233-server start
express233-server enable-autostart
express233-server set-port 32380
express233-server reset-root-password --password "change-me-now"
```

生产落地建议：

- Linux（Ubuntu / Debian / CentOS 7+）：优先用 `express233-server enable-autostart` 落到 `systemd`
- macOS：落到 `launchd`
- Windows：落到计划任务 `ONSTART`
- 开启原生自启动后，后续 `start` / `stop` / `restart` 会优先走原生服务控制面

## 一行部署

```bash
export EXPRESS233_SERVER=http://10.0.0.1:23380
export EXPRESS233_TOKEN=<token>
express233-cli deploy --project mygame --server-id game-logic-042 --dest /opt/game/042
```

批量：[examples/deploy-batch.csv](examples/deploy-batch.csv) + `express233-cli pull-batch --file ...`

列出中央已配置的 server_id：`express233-cli servers --server URL --token TOKEN`

环境自检：`express233-cli doctor`（检查 healthz、token、server_id、已发布版本）

版本回滚：`express233-cli rollback --server-id ID`（部署上一发布版；`--to 1.0.0` 指定版本）

版本 diff：`express233-cli diff --from 1.0.0 --to 1.1.0 --server-id ID`（对比两版本在 server_id 下的有效配置键）

审批流：operator 上传并「提交审批」→ admin「正式发布」或「驳回」

多租户：默认租户 `default`；`root` 登录后可 `POST /api/tenants` 创建新租户。每租户独立 `server.yaml` 与项目空间。

配置覆盖（`server.yaml`）示例——只覆盖必要字段：

```yaml
replacements:
  application.yaml:
    mysql:
      url: jdbc:mysql://db-a:3306/game
      password: secret
```

同名配置文件须为同一格式（`.yaml` 用嵌套对象，`.properties` 用扁平或嵌套展平为 dotted 键）。

## 本地脚本

**仓库根目录（Windows 双击 / 命令行）：**

| 脚本 | 说明 |
|------|------|
| [run-server.cmd](run-server.cmd) / [run-server.sh](run-server.sh) | 启动中央服 `:23380`（若本机已有 express233-server 会先自动停止） |
| [install-unicli.cmd](install-unicli.cmd) | 安装 [neko233-com/unicli](https://github.com/neko233-com/unicli)（端口/进程工具，推荐） |
| [install-gh.cmd](install-gh.cmd) | 安装 [GitHub CLI](https://cli.github.com/)（CI 日志、发布门禁） |
| [test-server.cmd](test-server.cmd) / [test-server.sh](test-server.sh) | `go test ./...` |
| [build-all.cmd](build-all.cmd) / [build-all.sh](build-all.sh) | 构建 CLI + Server 到 `bin/` |
| [git-deploy.cmd](git-deploy.cmd) / [git-deploy.sh](git-deploy.sh) | 本地模拟 CI（vet/测试/构建/冒烟）；`git-deploy.cmd --push` 再推送 |
| [visual-e2e.cmd](visual-e2e.cmd) / [visual-e2e.sh](visual-e2e.sh) | **浏览器全流程**（Playwright）；**不参与发布/ git-deploy** |

可视化验收技能：[.cursor/skills/express233-visual-verify/SKILL.md](.cursor/skills/express233-visual-verify/SKILL.md)

**子目录（同上逻辑）：**

| 目录 | 说明 |
|------|------|
| [run/run.sh](run/run.sh) / [run/run.cmd](run/run.cmd) | 启动 `express233-server`（`:23380`，数据目录 `EXPRESS233_DATA` 或 `.data`） |
| [test/test.sh](test/test.sh) / [test/test.cmd](test/test.cmd) | 运行 `go test ./...` |
| [deploy-github/deploy-github.sh](deploy-github/deploy-github.sh) / [.cmd](deploy-github/deploy-github.cmd) | 本地模拟 CI：vet、测试、构建、冒烟（需 Git Bash 跑 smoke） |

## 项目协作与邀请

- 创建项目后，创建者自动成为 **项目管理员**（读写）。
- 项目管理员在 Web「团队与邀请」生成链接，形如：`https://中央服务/#invite?token=...`
- 被邀请人登录同一租户账号后打开链接 → **接受邀请**。
- **只读成员**（`viewer`）：预览、列表、拉取已发布版本；不能上传/发布。
- **项目管理员**（`admin`）：在该项目内可上传、审批、发布。
- 租户级 `root` 可查看租户内全部项目；普通用户仅能看到已加入的项目。

运维：`GET /metrics`（Prometheus）、`GET /api/audit-logs`（管理员）、`scripts/backup-data.sh`

## Docker 本地中央服

```bash
docker compose up --build
# http://localhost:23380  root/root
```

## Cloudflare

Cloudflare Workers / Pages 不能直接运行 `express233-server` Go 二进制；推荐用 Worker 作为现有中央服的 HTTPS 反向代理，或 Pages 托管静态控制台并把 `/api/*` 代理到中央服。示例见 [docs/CLOUDFLARE.md](docs/CLOUDFLARE.md) 与 [examples/cloudflare/worker-proxy](examples/cloudflare/worker-proxy)。

## HTTP 自动化 Demo

管理端受保护 API 现在支持两种方式：

- `POST /api/login` 获取 Cookie / JWT
- 直接用 HTTP Basic Auth，例如 `root/root`

多版本、整包上传、`server_id` 注册与替换预览、diff、发布、拉取的完整 curl 演示见 [docs/HTTP_AUTOMATION_DEMO.md](docs/HTTP_AUTOMATION_DEMO.md)。

## Ansible 批量

[examples/ansible/](examples/ansible/)：`inventory.ini` 中为每台逻辑服设置 `express233_server_id` 与 `express233_dest`。

## 开发

```bash
make test
make build
make smoke      # 本地冒烟（同 CI）
make lint       # golangci-lint v2（.golangci.yml version: "2"）
make run-server # :23380 root/root
```

`run-server` 会设置 `EXPRESS233_WEB_DIR=internal/api/web`：开发时修改 `html`/`css`/`js` 后刷新浏览器即可，无需重启 Go 进程。

重复启动时脚本会探测 `http://127.0.0.1:23380/healthz`，仅停止进程名或命令行含 `express233-server` 的实例（不会误杀 `proxysss` 等）。跳过：`set EXPRESS233_NO_KILL=1`。

## GitHub Actions

| Workflow | 触发 | 作用 |
|----------|------|------|
| [ci.yml](.github/workflows/ci.yml) | push/PR | 三平台测试、vet、构建、冒烟、ShellCheck |
| [lint.yml](.github/workflows/lint.yml) | push/PR | golangci-lint |
| [codeql.yml](.github/workflows/codeql.yml) | push/PR/每周 | 安全扫描 |
| [release.yml](.github/workflows/release.yml) | tag `v*` / 手动 | 多平台 CLI + Server 发布 |
| [docker.yml](.github/workflows/docker.yml) | push/PR/tag | 构建并推送 `express233-server` 镜像 |
| [helm.yml](.github/workflows/helm.yml) | Helm 变更 | `helm lint` + template |

详见 [docs/GITHUB_ACTIONS.md](docs/GITHUB_ACTIONS.md)。

## 文档

- [AGENTS.md](AGENTS.md) — Agent 指南与 **New API 暗黑 UI** 规范
- [configs/server.yaml.example](configs/server.yaml.example) — 嵌套 `replacements`（按配置文件 basename）
- [configs/post-hook.yaml.example](configs/post-hook.yaml.example) — 拉取后处理（`when: os == ...` / `else`）
- 校验夹具：`testdata/validation-tree/`（双服、树形配置、拉取替换与 post-hook 集成测试）
- [docs/DEPLOY.md](docs/DEPLOY.md) — Helm / K8s / CLI 配置
- [docs/CLOUDFLARE.md](docs/CLOUDFLARE.md) — Cloudflare Worker / Pages 入口适配
- [docs/HTTP_AUTOMATION_DEMO.md](docs/HTTP_AUTOMATION_DEMO.md) — root/root + Basic Auth 自动化演示（多版本 / 模板替换 / 整包上传）
- [docs/openapi.yaml](docs/openapi.yaml) — OpenAPI 3（运行时 `/api/openapi.yaml`）
- [scripts/deploy-on-host.sh](scripts/deploy-on-host.sh)
- 在线 Swagger：`http://<central>/docs/`


Go **1.26** · MIT
