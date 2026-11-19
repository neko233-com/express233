# express233 — Agent 指南

## 生产信息与凭据安全

- 禁止把真实线上公网 IP、SSH 地址、账号、密码、私钥、Token、Webhook、Cookie、JWT 或数据库连接串写入 Git、源码、AGENTS.md、README、YAML、测试夹具、日志、截图或命令行参数。
- 线上地址与凭据只能通过 Secret、环境变量或本机未跟踪配置注入；Express233 节点配置只允许保存账号名与 Secret 引用，不得在可跟踪文件中保存真实密码。
- 命令、日志、测试证据和交付说明必须脱敏，不得回显可直接登录线上的完整信息；示例仅使用保留测试网段与 `example.invalid`。

## 产品定位

游戏服务器 **SSH Push / Pull Agent** 交付控制台：项目与不可变版本包、按 `server_id` 共享配置覆盖、配置预览、版本发布、目标部署、团队与审计。完整文档入口为 [docs/README.md](docs/README.md)。

## CI / GitHub / 发布（必读）

### 工具链

| 工具 | 安装 | 用途 |
|------|------|------|
| **gh** | [install-gh.cmd](install-gh.cmd) 或 `winget install GitHub.cli` | Actions 日志、Release、PR |
| **golangci-lint** v2 | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` | 与 `lint.yml` 一致 |
| **Node.js 24 LTS** | `.nvmrc`（`24`） | Vite+、无头验收、Actions JS 步骤 |

`gh auth login` 后执行 `gh auth status`，发布/查日志需 **workflow** scope。

### Node.js 24 LTS（强制）

- 所有 GitHub Actions workflow 设置 `env.FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true`。
- `test/browser`：`package.json` → `"engines": { "node": ">=24" }`；workflow 使用 `node-version: "24"`。
- **禁止**在 workflow 中使用 Node 20/22。

### golangci-lint

- 配置文件 **必须** 含 `version: "2"`（见 [.golangci.yml](.golangci.yml)）。
- CI 使用 `golangci/golangci-lint-action@v9` + `install-mode: goinstall`（与 `go 1.26` 工具链一致）。
- 本地：`golangci-lint run --timeout=5m` 或 `make lint`。

### 发布门禁（必须遵守）

**仅当**下列 GitHub Check 在**目标 commit** 上均为 `success` 时，才可打 tag / 发布：

- `golangci-lint`
- `test (ubuntu-latest)`、`test (windows-latest)`、`test (macos-latest)`
- `build binaries`
- `validate scripts`

流程：

1. 合并 `main` → 等待 [CI](https://github.com/neko233-com/express233/actions/workflows/ci.yml) + [Lint](https://github.com/neko233-com/express233/actions/workflows/lint.yml) 全绿。
2. 可选：`bash scripts/require-green-checks.sh <sha>`
3. `git tag vX.Y.Z && git push origin vX.Y.Z`
4. [release.yml](.github/workflows/release.yml) 的 `verify-checks` job 会再次校验；失败则**不**创建 Release 产物。

`visual-e2e` **不参与**发布门禁。

### Agent Skills（GitHub）

| 位置 | Skill |
|------|--------|
| 项目 | [.cursor/skills/express233-github/SKILL.md](.cursor/skills/express233-github/SKILL.md) |
| 全局 | `~/.cursor/skills/express233-github/SKILL.md` |

调试 CI 时可配合：`gh-fix-ci`、`gh-address-comments`（用户本机 Codex skills）。

## Web 控制台与项目维护

- 控制台源码位于 `frontend/`，使用 Vite+、React 19、React Compiler、React Router 和 TypeScript。Go 服务使用 `CGO_ENABLED=0` 构建并嵌入前端产物。
- 采用 Vercel Geist 暗色配色。主体黑灰，白色主按钮，蓝色用于焦点和选中态，语义色用于实际状态。颜色统一定义在 `frontend/src/style.css`。
- 保留项目侧栏、项目内操作页签、`data-testid`、深链接、键盘操作及移动端布局。保持对外 HTTP API、鉴权、CLI 和 OpenAPI 契约。
- 项目级 CodeGraph233 MCP 配置在 `.codegraph233.json`、`.codex/config.toml` 与 `.mcp.json`。先运行 `cg233_status`，缺失或过期时用 `cg233_index` 增量更新，然后按需查符号、调用和影响范围。
- 浏览器验收使用无头 Agent Browser，优先批处理并复用会话。`visual-e2e.cmd` / `visual-e2e.sh` 在独立数据目录验证控制台，独立于稳定版的六项 GitHub Check。
- 参考技能为 `.cursor/skills/express233-ui/SKILL.md`、`.cursor/skills/express233-visual-verify/SKILL.md` 和 `.cursor/skills/express233-github/SKILL.md`。

## 后端与测试

- 默认 CI 为 `CGO_ENABLED=0 go test ./...`。在 Windows PowerShell 7 中先设置 `$env:CGO_ENABLED = '0'`。
- 制品默认使用本地策略。后端以 `BlobStorage` 注入策略，详见 [制品存储](docs/ARTIFACT_STORAGE.md)。
- `ValidateBeforePublish` 执行服务端版本检查。浏览器验收独立运行。
- 本地服务入口为 `run-server.cmd` 或 `run-server.sh`，默认 `127.0.0.1:23380`。

## 节点部署（必读）

详见 [docs/SAFE_DEPLOY.md](docs/SAFE_DEPLOY.md)，核心设计：

### 安全部署流程（stop → swap → start）

远程 SSH 服务器上**不能直接覆盖运行中的二进制**。正确流程：

1. **Pull 到临时目录** `GAME_ROOT/.tmp/{server_id}/`（不影响正在运行的服务）
2. **Stop 旧服务** 读取 `run/server.pid`，校验 PID 仍属于目标实例后发送 SIGTERM，等待玩家数据与 MySQL Flush 完成并由进程自行退出；默认超时 180 秒，超时中止发布且不换包，SIGKILL 仅允许人工明确授权的应急操作
3. **Swap 文件** rsync --exclude logs/ --exclude run/（保留日志和 PID）
4. **Start 新服务** 执行 `scripts/restart.sh`

### 多 game-server 隔离

一台机器运行多个 game-server 实例时，每个 `server_id` 必须独立：

| 资源 | 路径 |
|------|------|
| 最终目录 | `GAME_ROOT/{server_id}/` |
| 临时目录 | `GAME_ROOT/.tmp/{server_id}/` |
| 日志目录 | `GAME_ROOT/{server_id}/logs/` |
| PID 文件 | `GAME_ROOT/{server_id}/run/server.pid` |
| 备份目录 | `GAME_ROOT/.backup/{server_id}/` |

### 设计约束（改动时必须考虑）

- `logs/` 和 `run/` 在部署替换时**不被删除**
- 正常发布默认禁止 SIGKILL；仅人工明确授权的应急操作可启用强杀
- 批量部署默认**串行**，避免同时停多个服
- 部署脚本**幂等**，重复执行安全
- 已发布版本可删除节省磁盘，丢失后重新 pull 同步
- 磁盘数据布局: `{dataDir}/userdata/{slug}/projects/{name}/{version}/`

