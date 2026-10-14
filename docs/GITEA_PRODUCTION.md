# Gitea 流水线接入

本仓库提供专用 `express233-ci` runner 和 Gitea 模板。当前模板需要适配后才能作为正式发布链路；它们不自动继承 GitHub Release 的六项门禁。

## Runner 环境

[runner-compose.yml](../deploy/gitea/runner-compose.yml) 使用 ephemeral runner、单并发；构建镜像固定 Go 1.26 与 Node.js 24。由 Secret 管理系统或本机未跟踪配置注入 Gitea 地址和注册 Token：

```bash
: "${GITEA_INSTANCE_URL:?inject the Gitea URL}"
: "${GITEA_RUNNER_REGISTRATION_TOKEN:?inject the runner registration secret}"
cd deploy/gitea
docker build -t express233-gitea-ci:1.26-node24 .
docker compose -f runner-compose.yml up -d
```

Runner 挂载宿主机 Docker socket，只应服务可信仓库。临时模式和单并发不会消除这个权限边界。配置不输出环境或凭据，不把 Secret 作为 CLI 参数或写入镜像层。

如实例要求 `DEFAULT_ACTIONS_URL=self`，需在实例内维护对应的 `actions/checkout@v4` 镜像。镜像基础层、系统包、Node 安装源和 Go 模块仍可能访问外网，不能把镜像一个 Action 视为完整离线构建。

## 当前模板边界

检查 [.gitea/workflows/ci.yml](../.gitea/workflows/ci.yml) 和 [release.yml](../.gitea/workflows/release.yml) 后再启用：

- 当前仍有 `npm --prefix frontend ci/build`，但控制台已经改为 Go embed 的原生静态资源，仓库没有该前端构建目录；这些步骤需要与当前构建方式对齐。
- CI 模板主要交叉构建二进制，没有三平台测试、lint、脚本校验的完整发布门禁。正式软件发布仍遵守 [六项 GitHub Check](GITHUB_ACTIONS.md)，不能用一个 Gitea package job 代替。
- Generic Registry 上传步骤当前通过 `curl --user` 展开账号与 Token，会进入进程参数；启用前需改为读取环境、经标准输入传给 HTTP 客户端，或使用受保护凭据文件。CI 日志打码不等于参数安全。
- 保持 Node 24，并为 JavaScript Actions 配置 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true`；不要假设安装 Node 24 就已覆盖 Action 的运行时选择。

这些是当前模板的待适配项，不代表已经完成可执行的生产发布验收。

## 包的命名与校验

模板使用仓库变量 `GITEA_URL`，Secret `GITEA_PACKAGE_USER` / `GITEA_PACKAGE_TOKEN`，发布到 Generic Package Registry：

```text
https://git.example.invalid/api/packages/<owner>/generic/express233/vX.Y.Z/express233-server-linux-amd64
```

预期产物为五个平台的 CLI / Server 共 10 个二进制，加一份 `SHA256SUMS`；当前 Gitea 模板不生成 GitHub Release 的 `PROVENANCE.json`。清单使用 `dist/` 路径：将下载资产放入独立目录下的 `dist/`，在其父目录执行 `sha256sum --check dist/SHA256SUMS`。私有包下载的认证同样由环境提供，不放在 URL 或 curl 参数中。

不要复用 tag 或依赖上传冲突保证整批原子性：逐文件 PUT 失败可能留下部分资产。只有全部文件存在、摘要通过、版本和 commit 对齐才可安装；运行中的控制台更新遵守 [DEPLOY.md](DEPLOY.md)。

## 游戏项目接入

[游戏项目模板](../examples/gitea/server-project-sf-go-express233.yml) 演示构建 Go main 包、上传归档、发布版本，再创建并轮询 SSH 部署。它针对特定游戏仓库：当前触发包括 `main/master` 分支和 `v*` tag，目标为 `server_ids: ["111"]` 与模板中的标签过滤；复制前必须替换项目、目标和发布条件，不可假定适用于所有游戏服。

凭据配置为 CI Secret：`EXPRESS233_URL`、`EXPRESS233_USERNAME`、`EXPRESS233_PASSWORD`。当前模板登录请求把包含密码的 JSON 展开为 `curl -d` 参数，接入前应改成 `--data-binary @-` 从标准输入读取；不能仅因使用 Cookie 就声称登录凭据不进入命令行。归档也必须显式排除本机 Secret、数据、日志和临时文件，检查打包清单后再上传。

CI 不需要游戏节点 SSH 密码；中央控制台负责 SSH。可跟踪节点配置只保留账号名和 Secret 引用，调用方从安全来源解析后通过 HTTPS 写入；当前主机 API 用 `credential` / `private_key` 接收实际凭据并加密保存，读取不返回原文。发布前先预览目标，保持串行、安全停服、数据保留与显式备份策略，详见 [SSH_PUSH_DEPLOY.md](SSH_PUSH_DEPLOY.md)。