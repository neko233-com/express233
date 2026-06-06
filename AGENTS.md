# express233 — Agent 指南

## 产品定位

游戏逻辑服 **拉模式部署** 中央控制台：项目管理、版本包、按 `server_id` 配置替换、拉取预览、发布与团队邀请。

## CI / GitHub / 发布（必读）

### 工具链

| 工具 | 安装 | 用途 |
|------|------|------|
| **gh** | [install-gh.cmd](install-gh.cmd) 或 `winget install GitHub.cli` | Actions 日志、Release、PR |
| **golangci-lint** v2 | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` | 与 `lint.yml` 一致 |
| **Node.js 24 LTS** | `.nvmrc`（`24`） | `test/visual`、Actions JS 步骤 |

`gh auth login` 后执行 `gh auth status`，发布/查日志需 **workflow** scope。

### Node.js 24 LTS（强制）

- 所有 GitHub Actions workflow 设置 `env.FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true`。
- `test/visual`：`package.json` → `"engines": { "node": ">=24" }`；workflow 使用 `node-version: "24"`。
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

## Web UI 设计规范（New API 暗黑风格）

所有 `internal/api/web/*` 改动必须遵循本规范，与 [New API](https://github.com/QuantumNous/new-api) / shadcn 暗色语义一致。

### 视觉基调

- **暗黑控制台**：深灰底 + 略亮侧栏 + 卡片浮层，禁止大面积纯白或 macOS 浅灰风。
- **主色**：emerald `#10b981`（贴近 New API），用于主按钮、选中态、焦点环；勿用过高饱和纯绿。
- **语义色**：成功 `#10b981`、警告 `#f59e0b`、危险 `#ef4444`、信息 `#3b82f6`。
- **字体**：12.5px 正文、紧凑行高；代码区 `ui-monospace`。
- **圆角**：控件 `6px`、卡片 `8px`（shadcn 默认量级）。
- **密度**：侧栏 232px；卡片/工具栏用 `--space-*` 间距，避免大面积留白。
- **禁止**：emoji 当图标、高饱和渐变背景、厚重阴影、浅色 Apple 风。

### 设计令牌（CSS 变量，定义于 `style.css`）

| 令牌 | 用途 |
|------|------|
| `--bg-base` | 主内容区背景 |
| `--bg-sidebar` | 左侧项目栏 |
| `--bg-surface` | 卡片 / 面板 |
| `--bg-elevated` | 输入框、代码块 |
| `--border` | 分割线、描边 |
| `--text` / `--text-muted` | 主文 / 次要文 |
| `--primary` | 主操作、选中项目 |
| `--sidebar-w` | 侧栏宽度（232px） |
| `--space-1` … `--space-4` | 紧凑间距阶梯 |

新增颜色只扩展变量，不要在组件里写死 hex（测试截图对比除外）。

### 布局结构（固定，勿随意改 DOM 语义）

```
登录全屏 (#login)
已登录 (#app.app-shell)
├── aside.sidebar — 项目搜索 + 列表 + 全局导航 + 用户区
└── main.main
    ├── #emptyProject — 未选项目
    ├── #globalServer — server.yaml
    ├── #globalSettings — 账号 / 审计
    └── #projectWorkspace — 选中项目后
        ├── 横向 .project-tabs（版本 / 预览 / 团队 / 部署 / 差异）
        └── .ptab-panel*
```

- **左侧**：项目列表 + 搜索（垂直 Tab 列表语义）。
- **项目内**：顶部横向 Tab 切换操作面，不把全部功能堆在一屏。

### 组件约定

- **按钮**：`.btn` + `.btn-primary` | `.btn-secondary` | `.btn-danger` | `.btn-ghost`；小号 `.btn-sm`。
- **输入**：`.input` / `.search-input`；聚焦 `box-shadow` 主色 25% 透明度。
- **卡片**：`.card`，细边框无大阴影。
- **表格**：`.data-table` / `.preview-table`，表头 `text-muted`，行 hover 微亮。
- **徽章**：`.badge` + `.badge-ok` | `.badge-draft` | `.badge-warn` 对应版本状态。

### 交互与无障碍

- 保留 `data-testid`（Playwright `test/visual` 依赖），改样式勿删 testid。
- 路由仍为 SPA 单页：`/` 未登录显示登录，JWT 在 `localStorage` + HttpOnly Cookie。
- 401 时清 token 并回到登录屏。

### 修改 Web 时检查清单

1. 暗色对比度：正文与背景对比 ≥ 4.5:1。
2. 侧栏选中态清晰可见（左边条 + 背景高亮）。
3. 横向 Tab 激活态有底边或背景区分。
4. 移动端：侧栏可收窄或堆叠（`@media max-width: 900px`）。
5. 跑 `test/visual` 或 `visual-e2e.cmd` 确认关键流程未断。

### 相关 Skill

- 控制台 UI：`.cursor/skills/express233-ui/SKILL.md`
- 浏览器验收：`.cursor/skills/express233-visual-verify/SKILL.md`
- GitHub / CI / 发布：`.cursor/skills/express233-github/SKILL.md`

## 后端与测试（简）

- Go 模块根目录；`go test ./...` 为默认 CI 测试（不含 `test/visual` npm 包）。
- 发布校验 `ValidateBeforePublish` **不**跑浏览器 E2E。
- 本地服务：`run-server.cmd` → `127.0.0.1:23380`（自动停止旧 express233-server 实例）。

## 节点部署（必读）

详见 [docs/SAFE_DEPLOY.md](docs/SAFE_DEPLOY.md)，核心设计：

### 安全部署流程（stop → swap → start）

远程 SSH 服务器上**不能直接覆盖运行中的二进制**。正确流程：

1. **Pull 到临时目录** `GAME_ROOT/.tmp/{server_id}/`（不影响正在运行的服务）
2. **Stop 旧服务** 读取 `run/server.pid`，SIGTERM → 超时 SIGKILL
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
- 批量部署默认**串行**，避免同时停多个服
- 部署脚本**幂等**，重复执行安全
- 已发布版本可删除节省磁盘，丢失后重新 pull 同步
- 磁盘数据布局: `{dataDir}/userdata/{slug}/projects/{name}/{version}/`

