---
name: express233-visual-verify
description: >-
  用真实浏览器（Playwright / playwright-cli）可视化验证 express233-server 控制台全流程。
  发布、git-deploy、ValidateBeforePublish 均不依赖本技能；仅在开发/验收时手动运行。
---

# express233 可视化验收

## 何时使用

- 改动了 `internal/api/web/*`、发布/预览/邀请相关 UI 或 API 行为后，需要**肉眼级**确认页面流程。
- **不要**在「正式发布」或 `git-deploy.cmd` 流程中强行加入本测试（已刻意隔离）。

## 一键跑 Playwright（推荐）

仓库根目录：

```bash
# Windows
visual-e2e.cmd

# Linux / macOS / Git Bash
./visual-e2e.sh

# 有界面调试
cd test/visual && npx playwright test --headed
cd test/visual && npx playwright test --ui
```

覆盖流程：登录 → 建项目/版本 → 上传配置 → 拉取预览（含右侧全文）→ 发布前检查 → 正式发布 → server.yaml 页签 → API 文档链接。

## 搭配 Cursor / MCP 浏览器

若已启用浏览器或 Playwright MCP，按此顺序人工/半自动核对：

1. `run-server.cmd` 或 `go run ./cmd/express233-server -addr :23380`
2. 打开 `http://127.0.0.1:23380/`
3. 使用页面 `data-testid` 定位（见 `internal/api/web/index.html`）：
   - `login-submit` → `app-shell`
   - `add-project` / `add-version` / `file-input`
   - `preview-server-id` / `preview-rendered-body`
   - `validate-version` / `publish-version`
4. 截图对比：预览右侧应显示替换后 YAML/properties 全文。
5. 发布后 `ver-status` 为 `published`，`download-version` 可见。

Playwright CLI（可选，见 `~/.codex/skills/playwright`）：

```bash
export PWCLI="$CODEX_HOME/skills/playwright/scripts/playwright_cli.sh"
"$PWCLI" open http://127.0.0.1:23380 --headed
"$PWCLI" snapshot
```

## 与发布的关系

| 环节 | 是否跑可视化 E2E |
|------|------------------|
| Web「发布前检查」`ValidateBeforePublish` | 否（仅目录/唯一 basename/server.yaml 引用） |
| `POST .../publish` | 否 |
| `git-deploy.cmd` / `deploy-github` | 否 |
| `go test ./...` | 否（测试在 `test/visual`，独立 npm 包） |
| GitHub `release.yml` | 否 |
| `visual-e2e.cmd` / workflow `Visual E2E` | 是（开发验收） |

## 故障排查

- 端口占用：设置 `EXPRESS233_VISUAL_PORT=39235` 与 `EXPRESS233_BASE_URL`。
- 复用已启动的服务：不设 `CI=1` 时 Playwright `reuseExistingServer` 为 true。
- 报告：`test/visual/playwright-report/` 或 `npx playwright show-report`。
