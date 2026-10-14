# 文档目录

[返回项目 README](../README.md) · [网页阅读入口](https://neko233-com.github.io/express233/) · [正式版本](https://github.com/neko233-com/express233/releases/latest)

## 按任务阅读

| 你要做什么 | 从这里开始 |
| --- | --- |
| 创建项目、上传包、发布和部署 | [控制台日常流程](CONSOLE_WORKFLOW.md) |
| 了解版本、配置、SSH 和 Pull 的关系 | [交付控制平面](DELIVERY_CONTROL_PLANE.md) |
| 安装中央服、升级并保留数据 | [部署与运维](DEPLOY.md) |
| 新增 SSH 机器、绑定区服、固定目标 | [SSH Push 部署](SSH_PUSH_DEPLOY.md) |
| 单服 / 多服停服换包 | [安全部署](SAFE_DEPLOY.md) |
| 编写启动、健康检查和回滚脚本 | [部署脚本契约](DEPLOYMENT_SCRIPT_CONTRACT.md) |
| 设置系统、架构与用途标签 | [文件标签](FILE_TAGS.md) |
| 从 CI 调用上传、预览与发布 API | [HTTP 自动化示例](HTTP_AUTOMATION_DEMO.md) |
| 接入 Gitea Actions | [Gitea 生产接入](GITEA_PRODUCTION.md) |
| 开发本项目、通过门禁并发布 Release | [GitHub Actions](GITHUB_ACTIONS.md) |
| 用 Cloudflare 暴露 HTTPS 入口 | [Cloudflare](CLOUDFLARE.md) |

## 参考与示例

- [OpenAPI](openapi.yaml)：运行时同一份规范位于 `/api/openapi.yaml`，Swagger UI 位于 `/docs/`。
- [分服配置示例](../configs/server.yaml.example)：`servers.<server_id>.replacements`，按 basename 覆盖。
- [post-hook 示例](../configs/post-hook.yaml.example)：交付后的脚本条件与参数。
- [Ansible 示例](../examples/ansible/) 与 [批量拉取列表](../examples/deploy-batch.csv)。
- [Cloudflare Worker 示例](../examples/cloudflare/worker-proxy/README.md)。
- [截图及再生成方法](../.assets/README.md)、[更新记录](../CHANGELOG.md)、[协作规范](../AGENTS.md)。

## 控制台内置指南

这些 Markdown 会编译进中央服，由 `/api/agent/guide` 列出主题，`/api/agent/guide/{topic}` 返回 JSON 中的 `content`。它们是公开静态说明，不返回真实项目、地址、账号或部署状态。

| 主题 | 源文件 |
| --- | --- |
| 版本与交付模型 | [overview](../internal/api/guide/overview.md) |
| Pull Agent | [agent](../internal/api/guide/agent.md) |
| Gitea 自动化 | [gitea](../internal/api/guide/gitea.md) |
| GitHub 自动化 | [github](../internal/api/guide/github.md) |
| 安全边界 | [security](../internal/api/guide/security.md) |

## 阅读时需要区分

1. **发布版本包**锁定文件，并不等同于游戏服已经更新；自动 Hook 或 Pull 跟随可能另行触发交付。
2. **控制台预演**只解析目标，不连接 SSH、不停服，不验证远端服务是否健康。
3. **配置覆盖**按租户共享，在拉取时应用；版本包冻结不会冻结 `server.yaml`。回滚历史版本仍需核对当前分服配置。
4. **CLI 与安全 runner**职责不同：`pull` / `deploy` 都直接解包到目标目录，默认执行 post-hook；安全换包应使用 runner。Linux 与 Windows 的停进程保证不同，先阅读安全部署中的平台边界。
5. **验证证据**分层记录：CI 和浏览器流程证明控制台行为，真实游戏服停服、存盘、启动及玩家链路需要目标环境验收。

## 文档维护

界面文案以 `internal/api/web/` 为准，命令与默认值以 `cmd/`、`scripts/` 为准。修改 API 说明时同步 `docs/openapi.yaml` 与 `internal/api/openapi.yaml`；目前这两份文件保持一致，没有单独的代码生成器。界面变化后重新运行截图任务，不使用生产截图作为公开示例。

`docs/index.html` 是 GitHub Pages 的静态阅读入口；源码文档链接回 GitHub，避免 Pages `/docs` 发布根目录之外的相对路径失效。内置指南变更需要随中央服新版本发布。
