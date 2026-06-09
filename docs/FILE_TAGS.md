# 文件标签与选择性拉取

一个版本可以同时上传 Windows、macOS、Linux，以及 x86/arm64 的全部产物。文件标签用于让下载方只拉取自己需要的文件，避免跨平台二进制浪费流量。

## 标签规则

- 未设置标签等价于 `all`（内部兼容 `*`），表示所有下载方都会拉取。
- 下载方匹配任一标签即可下载该文件。
- 新版 CLI 会自动上报：
  - `os`: `runtime.GOOS`，如 `linux`、`windows`、`darwin`
  - `arch`: `runtime.GOARCH`，如 `amd64`、`arm64`
  - 组合标签：服务端会匹配 `os-arch`，如 `linux-amd64`
- 额外标签可用 `--tag` 或 `tags` query 传入，用于 `canary`、`gpu`、`region-a` 等运维维度。

推荐：

| 文件 | 标签 |
|------|------|
| `config/application.yaml` | `all` |
| `scripts/restart.sh` | `linux` |
| `bin/game-server-linux-amd64` | `linux-amd64` |
| `bin/game-server-linux-arm64` | `linux-arm64` |
| `bin/game-server-windows-amd64.exe` | `windows-amd64` |
| `bin/game-server-darwin-arm64` | `darwin-arm64` |

## CLI

```bash
express233-cli pull \
  --server http://central:23380 \
  --project mygame \
  --server-id game-logic-01 \
  --token "$EXPRESS233_TOKEN" \
  --dest /opt/game/game-logic-01
```

账号密码方式：

```bash
EXPRESS233_USERNAME=root EXPRESS233_PASSWORD=root \
express233-cli pull --server http://central:23380 --project mygame --server-id game-logic-01
```

默认会自动带当前机器的 `os/arch`。

手动指定：

```bash
express233-cli pull --os linux --arch arm64 --tag gpu ...
```

## API

列出标签：

```http
GET /api/projects/{id}/versions/{ver}/file-tags
```

设置单个文件：

```http
PUT /api/projects/{id}/versions/{ver}/file-tags
Content-Type: application/json

{"path":"bin/game-server-linux-amd64","tags":["linux-amd64","linux"]}
```

批量追加：

```http
POST /api/projects/{id}/versions/{ver}/file-tags/batch
Content-Type: application/json

{
  "patterns": ["bin/linux/**", "scripts/*.sh"],
  "tags": ["linux"],
  "mode": "add"
}
```

`mode` 支持 `set`、`add`、`remove`、`clear`。
