# GitHub Actions 接入

GitHub Actions 的职责与 Gitea 相同：构建跨平台制品，使用 Secret 中的最小权限凭据调用 express233 API，并发布不可变版本。

建议在工作流中：

1. 构建并检查二进制、嵌套配置、启动与健康检查脚本。
2. 创建明确的语义化版本，上传文件和标签。
3. 调用发布校验并发布。
4. 观察 Hook 事件和发布日志，而不是在 CI 中保存 SSH 密码。

当前项目的 GitHub CI 只打包与静态检查；完整测试在合并前和本地发布验收中执行，以缩短制品发布时间。
# GitHub Actions：可复现制品溯源

构建 workflow 应将 `${{ github.repository }}`、`${{ github.ref }}`、`${{ github.sha }}`、`${{ github.run_id }}` 和 run URL 作为 `vcs` 随创建版本请求提交。express233 自己的 CI/Release artifact 同时包含 `PROVENANCE.json` 与 `SHA256SUMS`。

Tag workflow 使用 tag 作为版本名；分支 workflow 发送空 `name` 以让服务端原子分配下一个 patch，并使用 API 响应的 `version`。这避免并发 GitHub Actions 运行得到相同版本号。

仓库 URL 必须是无凭据 HTTPS 地址。不要把 PAT、部署密码、SSH 私钥或数据库连接放进 VCS 元数据、制品路径或日志。
