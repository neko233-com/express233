---
name: express233-ui
description: >-
  优化 express233 Web 控制台 UI/UX。遵循 AGENTS.md 与 New API 暗黑风格：深灰侧栏、翠绿主色、
  左项目栏 + 内横向 Tab。修改 internal/api/web 前必读。
---

# express233 控制台 UI

## 何时使用

- 用户抱怨 UI 丑、难用、要暗黑 / New API 风格。
- 改 `internal/api/web/index.html`、`style.css`、`app.js` 的布局或视觉。

## 必读

1. 仓库根目录 [AGENTS.md](../../AGENTS.md) — 设计令牌、布局、禁止项。
2. 勿破坏 `data-testid`（见 `test/visual/tests/server-flows.spec.ts`）。

## New API 暗黑要点（执行清单）

- 背景：`#09090b` 主区，`#0f0f11` 侧栏，`#18181b` 卡片（zinc 暗色）。
- 边框：`rgba(255,255,255,0.06)`，不用黑色硬边。
- 主色：`#10b981`；悬停 `#34d399`。
- 文字：主 `#fafafa`，次 `#a1a1aa`（`--text-muted`）。
- 紧凑：12.5px 正文、`--space-*` 间距、侧栏 232px。
- 侧栏项：hover 微亮底；选中 = 左侧 3px 主色条 + `rgba(34,197,94,0.12)` 底。
- 项目 Tab：底部 2px 主色指示条，非「白底蓝字」。
- 登录：居中暗色卡片 + 弱光晕，禁止浅色 Apple 卡片。

## 文件职责

| 文件 | 职责 |
|------|------|
| `index.html` | 结构、侧栏、Tab、`data-testid` |
| `style.css` | 全部视觉（仅改变量 + 类） |
| `app.js` | 逻辑；样式用 class 而非内联 |

## 禁止

- 引入 React/Vue 构建链（保持静态 embed）。
- 删除 JWT / 登录分流逻辑。
- 把发布流程绑进 Playwright 默认 CI。

## 验收

```cmd
run-server.cmd
visual-e2e.cmd
```

浏览器：登录 → 建项目 → 版本 → 预览 → 发布；侧栏与 Tab 切换无错位。
