# GitHub Actions 说明

## 工作流与工具链

| 工作流 | 触发与验证范围 |
|---|---|
| `ci.yml` | `main` / `master` push、PR；Ubuntu / Windows / macOS 的 Go 测试、五平台 CLI/Server 构建、已跟踪 Bash/PowerShell 脚本语法 |
| `lint.yml` | 同 CI；`golangci-lint` v2，action v9 + `install-mode: goinstall` |
| `visual-e2e.yml` | 手动，或匹配 Web / 浏览器测试路径的 PR；无头 Agent Browser 验证打包后的控制台 |
| `codeql.yml` | push、PR、每周一 06:00 UTC；Go 安全分析 |
| `helm.yml` | `main` / `master` push、PR 中 `deploy/helm/**` 变更；lint 与模板渲染 |
| `docker.yml` | push、PR、`v*` tag；PR 只构建，其它触发可推送镜像 |
| `docker-integration.yml` | 手动；Docker 集成验证 |
| `release.yml` | `v*` tag 或手动；六项门禁通过后构建和发布二进制 |

工作流使用 Go 1.26.x 与 `CGO_ENABLED=0`。CI 和 Release 在编译 Go 服务端前用 Vite+ 构建控制台并嵌入静态产物。Docker 镜像同样先构建前端，再编译并拷贝纯 Go 二进制。

所有 GitHub workflow 设置 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true`；浏览器测试显式使用 Node.js **24 LTS**，本地版本见 `.nvmrc`。禁止降到 Node 20/22。`.golangci.yml` 使用 `version: "2"`。Dependabot 的更新配置在 `.github/dependabot.yml`，不是一个 Actions workflow。

## 发布门禁

必须在**准备发布的同一 commit** 上看到以下六个 Check 全部 `success`，才能打 tag：

- `golangci-lint`
- `test (ubuntu-latest)`
- `test (windows-latest)`
- `test (macos-latest)`
- `build binaries`
- `validate scripts`

合并到 `main` 后等待这些结果；排队、运行中、跳过或其它 commit 成功都不算通过。`scripts/require-green-checks.sh` 查询目标 SHA 的 Check，缺失、非成功或 API 查询失败都会中止。`visual-e2e` 不属于这六项硬门禁，但 Web 改动仍需按仓库要求做浏览器验收。

```bash
# gh 使用已有安全认证；GH_TOKEN 如需提供，应由 CI Secret 注入。
TARGET_SHA=$(git rev-parse HEAD)
bash scripts/require-green-checks.sh "$TARGET_SHA"
: "${RELEASE_TAG:?set the intended vX.Y.Z tag}"
git tag "$RELEASE_TAG" "$TARGET_SHA"
git push origin "$RELEASE_TAG"
```

Release 的 `verify-checks` 会再次校验 tag 指向的提交，通过后才构建。手动重建时输入已有 tag，产物版本与提交信息取自实际检出的 tag。正式发布优先使用 tag push，每次分配独立版本号。

## Release 资产与验证

五个平台为 `linux/amd64`、`linux/arm64`、`darwin/amd64`、`darwin/arm64`、`windows/amd64`。每个平台提供两个二进制：

- `express233-cli-{os}-{arch}[.exe]`
- `express233-server-{os}-{arch}[.exe]`

另有 `safe-deploy.sh`、`batch-deploy.sh`、`PROVENANCE.json` 和 `SHA256SUMS`，合计 **14 个资产**。构建 job 生成一个 `dist` artifact，发布 job 下载后交给 `softprops/action-gh-release`。`PROVENANCE.json` 记录版本、repository、ref、commit 和运行来源；`SHA256SUMS` 覆盖二进制、节点脚本及 provenance，自身不在清单内。CLI Shell 安装器从同一固定版本下载全部所需资产，校验全部摘要后才更新目标路径。

下载到隔离目录后先校验，再安装。当前清单中文件名前缀是 `dist/`，不能把全部文件平铺在当前目录后直接校验：

```bash
: "${RELEASE_TAG:?set an existing release tag}"
RELEASE_DIR=$(mktemp -d)
gh release download "$RELEASE_TAG" --repo neko233-com/express233 --dir "$RELEASE_DIR/dist"
(
  cd "$RELEASE_DIR"
  sha256sum --check dist/SHA256SUMS
)
# 在匹配的 Linux amd64 主机上核对版本与 commit；其它平台选择对应资产。
chmod +x "$RELEASE_DIR/dist/express233-server-linux-amd64"
"$RELEASE_DIR/dist/express233-server-linux-amd64" version
```

还需比对 tag 的 Git commit、provenance 的 commit 与二进制输出；摘要一致只证明下载与该清单一致，不能替代发布来源和版本核对。安装或在线更新遵守 [部署指南](DEPLOY.md)，不直接覆盖正在运行的程序。

Docker 镜像路径由 workflow 定义为 `ghcr.io/<owner>/<repository>/express233-server`，tag 发布的 semver 镜像标签去掉 `v` 前缀。Docker workflow 独立运行，镜像构建成功不等于二进制 Release 或线上升级成功。

## 游戏项目版本与本地检查

Express233 软件 Release 使用 Git tag；上传到控制台的游戏项目版本是另一套 API。创建游戏项目版本时 `name` 可留空，由服务端事务分配下一个 patch；后续上传、发布、部署都使用响应中的 `version`。如有构建来源，同时提交 `vcs` 字段；不要把该 API 行为理解为 GitHub 自动打 tag。

```bash
CGO_ENABLED=0 go test ./... -count=1 -timeout=10m
CGO_ENABLED=0 golangci-lint run --timeout=5m
npm --prefix frontend run check
bash visual-e2e.sh
make smoke
```

Linux CI 使用纯 Go 重复执行部署事务的并发测试。脚本语法、Go 行为、浏览器验收和线上业务分别记录验证结果。
