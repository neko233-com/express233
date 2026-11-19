---
name: express233-ui
description: 维护 Express233 React 控制台、Geist 暗色配色、路由和 UI 行为。
---

# Express233 控制台

源码在 `frontend/src`。先阅读 [AGENTS.md](../../../AGENTS.md)，按影响范围使用项目 CodeGraph233 MCP 查询路由、组件和 API 调用。控制台使用 Vite+、React 19、React Compiler、React Router 与 TypeScript。

- 使用 `style.css` 的黑、灰、白设计令牌。白色用于主操作，蓝色用于选中和焦点，绿、红、琥珀用于语义状态。
- 保留侧栏项目列表和工作区页签语义。新页签使用路由，并验证深链刷新。
- 通过 `api.ts` 访问原有 HTTP API，保持鉴权、上传、错误和发布契约。
- 保留 `data-testid`、键盘操作，以及移动端和长输出的滚动边界。

从仓库根目录运行 `npm --prefix frontend run check`，然后执行 `visual-e2e.cmd` 或 `bash visual-e2e.sh`，用无头 Agent Browser 验证打包后的 Go 服务。详见 [浏览器验收](../express233-visual-verify/SKILL.md)。
