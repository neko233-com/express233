# GitHub Actions 说明

## 工作流一览

### CI (`ci.yml`)

- **触发**：`main` / `master` 的 push 与 PR
- **内容**：
  - `build binaries`：Ubuntu / Windows / macOS 构建 React 控制台与 CLI + Server，并上传平台二进制。
  - 默认 CI 刻意不跑单测、冒烟或脚本检查，以缩短构建/发布反馈；完整 Docker 集成验证见 `docker-integration.yml`，由人工触发。

### Lint (`lint.yml`)

- **触发**：同 CI
- **内容**：`golangci-lint`（配置见 `.golangci.yml`）

### CodeQL (`codeql.yml`)

- **触发**：push/PR + 每周一 06:00 UTC
- **内容**：Go 语言安全分析

### Release (`release.yml`)

- **门禁**：首个 job `verify-checks` 调用 `scripts/require-green-checks.sh`，要求该 commit 上 CI + Lint 全部 success，否则**中止**发布。
- **触发**：
  - 推送 tag `v*`（如 `v0.1.0`）
  - `workflow_dispatch`（手动重建，需在输入框填写已有 tag 名）
- **产物**（每个平台各 2 个二进制 + sha256）：
  - `express233-cli-{os}-{arch}[.exe]`
  - `express233-server-{os}-{arch}[.exe]`
- **VCS 溯源**：每个 CI/Release artifact 都附带 `PROVENANCE.json`（repository、ref、commit、run URL/ID）和 `SHA256SUMS`。将游戏/数据项目上传到 express233 时，应同时在创建版本请求中传入同一组 `vcs` 字段；发布后它们与服务端生成的制品清单摘要一同冻结。
- **版本自增**：tag 构建传入 tag 作为 `name`；分支构建将 `name` 留空，服务端会在 SQLite 事务内生成下一个 patch。必须使用响应中的 `version` 执行后续上传、发布和部署，避免多个并发 Runner 发生版本号竞争。
- **发布**：合并 matrix 产物后由 `softprops/action-gh-release` 创建 GitHub Release

### Helm (`helm.yml`)

- **触发**：`deploy/helm/**` 变更
- **内容**：`helm lint` + `helm template` 渲染检查

### Docker (`docker.yml`)

- **触发**：push/PR/tag
- **镜像**：`ghcr.io/<owner>/express233-server`
- **说明**：PR 仅 build 不 push；合并到 main 或打 tag 后推送

### Dependabot (`dependabot.yml`)

- 每周检查 Go 模块与 GitHub Actions 版本更新

## Node.js 24

- 所有 workflow 设置 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true`。
- `visual-e2e` 使用 Node **24** LTS；本地见仓库根 `.nvmrc`。

## 发布 checklist

1. 确保 `main` 上 **build binaries + Lint** 全部通过（Release 会二次校验）
2. 可选：`bash scripts/require-green-checks.sh $(git rev-parse HEAD)`
3. 打 tag：`git tag v0.1.0 && git push origin v0.1.0`
4. 等待 Release workflow 完成，检查 Assets
5. 节点使用 `install.sh` 安装 CLI；中央机使用 `install-server.sh` 或 Docker

## 本地对齐 CI

```bash
make test-race
make smoke
golangci-lint run --timeout=5m
bash scripts/require-green-checks.sh "$(git rev-parse HEAD)"  # 发布前
```

`.golangci.yml` 须为 **v2** 格式（首行 `version: "2"`）。
