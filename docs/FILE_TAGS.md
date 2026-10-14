# 文件标签与选择性拉取

同一版本可以保存多平台产物，文件标签决定下载方拿到哪些文件。它和 SSH 部署任务的“发布标签”不同：发布标签选机器，文件标签选包内文件。

## 匹配规则

- 未设置标签、空标签、`all` 或 `*` 均表示所有下载方可获取该文件。标签统一转为小写，逗号、空白和分号可分隔多个标签。
- 文件任一标签匹配下载方即可下载，是 **OR** 关系。多个标签不是“必须同时满足”的条件。
- CLI 默认发送当前 `GOOS` 和 `GOARCH`；服务端还匹配组合标签，例如 Linux amd64 下载方会匹配 `linux`、`amd64`、`linux-amd64`。
- `pull/deploy --tag` 可重复指定额外标签；HTTP 对应可重复的 `tags` query 参数。

推荐按以下方式标记：

| 文件 | 标签 |
|---|---|
| `config/application.yaml` | `all` |
| `scripts/restart.sh` | `linux` |
| `scripts/restart.ps1` | `windows` |
| `bin/game-server-linux-amd64` | `linux-amd64` |
| `bin/game-server-linux-arm64` | `linux-arm64` |
| `bin/game-server-windows-amd64.exe` | `windows-amd64` |
| `bin/game-server-darwin-arm64` | `darwin-arm64` |

不要给仅限 Linux amd64 的二进制同时设置 `linux` 和 `amd64`，也不要设置 `linux-amd64,linux`：这样其他 Linux 架构也会匹配。`linux-amd64,gpu` 同样表示任一匹配，不表示“Linux amd64 且有 GPU”。

## 在控制台设置

在草稿版本中，上传前填写默认标签；对 ZIP / TAR 上传设置的标签会应用到归档内文件，因此混合平台包建议先上传，再按文件设置标签。“高级：批量文件标签与构建来源”中的批量工具用于已有文件。

当前 HTTP 标签修改入口只接受草稿；待审、驳回和已发布状态的修改请求会被拒绝。修改已发布包的分发规则应创建新版本，再检查、发布。按当前版本与 `server_id` 的配置预览主要用于核对配置替换，不能代替验证每种平台实际拉取到的文件集合。

## CLI 示例

以下示例只下载到独立 staging 目录，并跳过后处理。先由 Secret 或受控运行环境注入 `EXPRESS233_TOKEN`，或 `EXPRESS233_USERNAME` / `EXPRESS233_PASSWORD`；不把凭据拼到命令行中。

```bash
express233-cli pull \
  --server https://control.example.invalid \
  --project game-logic \
  --server-id logic-021 \
  --version 1.4.3 \
  --dest /opt/game-staging/logic-021 \
  --skip-hook
```

默认使用当前机器平台。检查另一平台时显式指定，并保持隔离目录：

```bash
express233-cli pull \
  --server https://control.example.invalid \
  --project game-logic \
  --server-id logic-021 \
  --version 1.4.3 \
  --os linux --arch arm64 \
  --tag gpu \
  --dest /opt/game-staging/logic-021-linux-arm64 \
  --skip-hook
```

省略 `--version` 时取最新已发布版本，按发布时间而非版本号大小确定。`server_id` 必须存在共享配置条目。普通 `pull` / `deploy` 默认执行配置的后处理，只有 `--skip-hook` 会跳过；二者不执行完整的停服换包保护。实际部署使用安全脚本或 Agent，见[交付控制平面](DELIVERY_CONTROL_PLANE.md)。

## HTTP API

读取需要项目访问权限，修改需要项目管理权限，并使用管理 JWT / Basic 会话。Pull Token 不能替代管理接口认证。

| 方法和路径 | 说明 |
|---|---|
| `GET /api/projects/{id}/versions/{ver}/file-tags` | 列出每个文件及标签 |
| `PUT /api/projects/{id}/versions/{ver}/file-tags` | 设置一个文件的完整标签 |
| `DELETE /api/projects/{id}/versions/{ver}/file-tags?path=...` | 清除指定文件标签，恢复所有平台可取 |
| `POST /api/projects/{id}/versions/{ver}/file-tags/batch` | 按 `paths` 或 `patterns` 批量操作 |

设置单文件，请求体为：

```json
{"path":"bin/game-server-linux-amd64","tags":["linux-amd64"]}
```

批量限定 Linux 脚本，请求体为：

```json
{
  "patterns": ["scripts/*.sh"],
  "tags": ["linux"],
  "mode": "set"
}
```

`paths` 使用精确相对路径；`patterns` 支持普通 glob 及 `目录/**` 递归前缀，例如 `config/**`。不要假设支持任意位置的 globstar 表达式。两者合并选择已有文件，响应的 `updated` 列出更新项，无匹配时为 `[]`。

| `mode` | 含义 |
|---|---|
| `set` | 完整替换标签，默认模式 |
| `add` | 加入标签；原来为 `all` 的文件仍为全部可取 |
| `remove` | 删除指定标签；删空后恢复全部可取 |
| `clear` | 清除限制，恢复全部可取 |

要把默认 `all` 改为仅 Linux，必须用 `set`；`add linux` 不会缩小范围。`clear` 不会删除文件，也不表示禁止下载。
