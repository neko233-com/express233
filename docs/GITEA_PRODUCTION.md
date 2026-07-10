# Gitea 生产流水线

本仓库的 Gitea 流水线不是 GitHub Actions 的降级副本：`.gitea/workflows/ci.yml` 固定使用专用 `express233-ci` runner；标签发布会将五个平台的 CLI/Server 二进制和 `SHA256SUMS` 上传到 Gitea Generic Package Registry。

## 一次性部署 runner

在受控 CI 主机执行：

```bash
cd deploy/gitea
docker build -t express233-gitea-ci:1.26-node24 .
export GITEA_INSTANCE_URL=https://git.example.com
export GITEA_RUNNER_REGISTRATION_TOKEN='从 Gitea 管理页生成的短期 runner token'
docker compose -f runner-compose.yml up -d
```

在 Gitea 实例配置中将 `DEFAULT_ACTIONS_URL` 设为 `self`，并镜像 `actions/checkout@v4` 至实例的 `actions/checkout` 仓库；这样生产 runner 不依赖 GitHub 网络。runner 使用 ephemeral 模式和单并发，避免不同发布任务共享工作目录或 Docker 凭据。不要给不可信仓库分配该 runner。

## 包发布与节点下载

在仓库变量设置 `GITEA_URL`，并创建只有 Package 写入权限的机器人账号及 `GITEA_PACKAGE_USER`、`GITEA_PACKAGE_TOKEN` secrets。tag `vX.Y.Z` 后，包的下载地址为：

```text
https://git.example.com/api/packages/<owner>/generic/express233/vX.Y.Z/express233-server-linux-amd64
```

下载后必须执行 `sha256sum --check SHA256SUMS`。Gitea Generic Registry 使用 HTTP PUT 逐文件上传，冲突版本会失败，确保发布不可变。
