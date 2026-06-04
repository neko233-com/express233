# GitHub Actions 说明

## 工作流一览

### CI (`ci.yml`)

- **触发**：`main` / `master` 的 push 与 PR
- **内容**：
  - `test`：Ubuntu / Windows / macOS 上 `go vet` + 单测（Ubuntu 含 `-race` 与 coverage 产物）
  - `build`：注入版本信息后编译 CLI + Server，执行 `scripts/ci-smoke.sh` 端到端冒烟
  - `scripts`：ShellCheck + `bash -n` 校验安装/部署脚本

### Lint (`lint.yml`)

- **触发**：同 CI
- **内容**：`golangci-lint`（配置见 `.golangci.yml`）

### CodeQL (`codeql.yml`)

- **触发**：push/PR + 每周一 06:00 UTC
- **内容**：Go 语言安全分析

### Release (`release.yml`)

- **触发**：
  - 推送 tag `v*`（如 `v0.1.0`）
  - `workflow_dispatch`（手动重建，需在输入框填写已有 tag 名）
- **产物**（每个平台各 2 个二进制 + sha256）：
  - `express233-{os}-{arch}[.exe]`
  - `express233-server-{os}-{arch}[.exe]`
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

## 发布 checklist

1. 确保 `main` 上 CI / Lint / CodeQL 通过
2. 打 tag：`git tag v0.1.0 && git push origin v0.1.0`
3. 等待 Release workflow 完成，检查 Assets
4. 节点使用 `install.sh` 安装 CLI；中央机使用 `install-server.sh` 或 Docker

## 本地对齐 CI

```bash
make test-race
make smoke
golangci-lint run ./...
```
