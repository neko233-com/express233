# express233 统一交付控制平面

express233 为游戏网关、逻辑服和战斗服管理项目版本包、共享分服配置及部署记录。日常操作见[控制台流程](CONSOLE_WORKFLOW.md)；这里说明 Push / Pull 的协议与边界。

## 版本发布与服务器部署

版本按项目隔离，状态包括草稿、待审、驳回和已发布。当前 HTTP 修改入口只接受草稿，已发布版本可拉取。正式版本名使用 `X.Y.Z`。发布时记录文件内容清单的 SHA-256，它不是上传 ZIP / TAR 的压缩包摘要。

最新已发布版本按 `published_at DESC, id DESC` 选择。创建顺序、语义版本大小、控制台当前选中版本均不改变这一规则。已发布包可以被删除或由保留策略清理；固定版本失效后不能自动替换成最新版本。

分服配置以租户内 `server_id` 共享，在拉取时应用到版本包；它没有随版本冻结。无分服差异可以跳过编辑与预览，但实际拉取和 Agent 心跳仍需已有的配置条目。配置列表不能用于推断某个项目的部署目标。

发布版本只开放版本包使用；启用的 Release Hook 和 Pull 自动跟随会在发布后推动部署。应在发布前明确自动规则，而不是把发布接口成功理解为所有游戏服已经上线。

## Push 与 Pull

| 维度 | SSH Push | Pull Agent |
|---|---|---|
| 控制入口 | 部署最新版本、可复用任务、Release Hook | 节点心跳及期望版本 |
| 网络方向 | 中央服主动 SSH 到目标机器 | 节点主动请求中央服 |
| 目标来源 | 明确绑定到本项目的 SSH Host + `server_id` | 认证心跳注册的项目 + `server_id` |
| 版本状态 | 最近成功的真实部署版本，排除预演 | `current_version`、`desired_version` 与 generation |
| 适用情况 | 中央服能访问机器，集中操作 | NAT 内节点、主动拉取 |
| 执行器 | 远端 Linux `safe-deploy.sh` | 节点本地 `safe-deploy.sh` 或 `.ps1` |

旧版单次 `/api/pull` 下载也会出现在 Pull 清单中。这种记录只能证明服务端完成打包响应，不证明下载方已启动游戏或通过健康检查；常驻 Agent 的 runner 回报才提供部署进度。

CLI 的 `deploy` 是 `pull` 的别名：下载、解压到 `--dest`，再按配置执行可选后处理；只有 `--skip-hook` 会关闭后处理。它不自动执行安全的停服换包流程。运行中实例应通过安全部署脚本或 Agent 先拉到临时目录，不能把普通 `pull/deploy --dest` 指向正在运行的二进制目录。

## SSH 目标、预演和幂等

“部署最新版本”使用当前项目全部明确绑定的 SSH 目标，确认框逐个列出 `server_id` 与机器名称；Pull 节点不在操作范围。未归属项目的历史绑定不会自动计入项目目标。

高级任务可以保留固定 `server_id` 子集，或在每次运行时读取项目当前绑定；两种模式均继续应用任务标签过滤，默认 `test`，支持 `all` / `any`。文件内容标签是另一套机制，见[文件标签](FILE_TAGS.md)。任务版本为空表示执行时取最新，非空必须是仍可用的已发布版本。UI 在固定版本失效时要求显式重新选择。

主要接口（写入需项目管理权限，读取需项目访问权限）：

| 方法和路径 | 用途 |
|---|---|
| `POST /api/projects/{id}/push-deployments` | 创建一次部署或预演 |
| `GET /api/projects/{id}/push-deployments/{deploymentID}` | 读取状态与逐服输出 |
| `POST /api/projects/{id}/push-tasks` | 保存可复用任务 |
| `PUT /api/projects/{id}/push-tasks/{taskID}` | 更新任务定义 |
| `POST /api/projects/{id}/push-tasks/{taskID}/run` | 按保存的范围运行；可传 `dry_run` |

一次项目范围预演的请求体示例：

```json
{
  "version": "1.4.3",
  "all_project_targets": true,
  "dry_run": true,
  "idempotency_key": "ci:game-logic:run-42:preview"
}
```

`all_project_targets` 不能与 `server_ids` 或 `tags` 同传。未使用它且未传标签时，按 `test` 筛选。`dry_run` 只解析并记录目标，**不连接 SSH、不传包、不停服、不检查机器健康**；返回 `success` 表示预演完成。列表和详情的 `selector` 是 JSON 字符串，使用其中的 `dry_run` 判断执行类型，不要仅按状态判定上线成功。

正式执行使用 `dry_run: false`，并使用与预演不同的幂等键。相同操作网络重试复用原键，接续原执行；修改版本、范围或执行类型应使用新键。键可通过 `Idempotency-Key` 头或请求体传入，同时传时必须一致。任务 `/run` 还可用非空 `version` 覆盖此次任务版本，CI 应避免无意覆盖固定版本策略。

## Pull Agent 协议

Agent 调用 `POST /api/agent/nodes/heartbeat`，服务端支持 `X-Express233-Token`、HTTP Basic Auth 或 JWT。CLI Agent 使用环境变量中的 Pull Token，或账号密码；认证信息不应放进命令行参数。

心跳包含 `project`、`server_id`、角色、环境、标签、平台、`current_version`、`applied_generation`、状态、心跳间隔与受限 `error_code`。不要上传原始命令输出或凭据。服务端校验项目访问权限以及 `server_id` 是否已配置。

返回体中的期望状态示例：

```json
{
  "desired": {
    "version": "1.4.3",
    "generation": 12,
    "needs_deploy": true
  }
}
```

- 使用 `PUT /api/projects/{id}/delivery-nodes/{serverID}/desired` 设置 `version` 与 `auto_follow`。指定版本必须已发布；开启自动跟随且版本为空时，会选择当前最新已发布版本。
- 期望版本改变时 generation 增加；重复设置同一版本不增加。仅修改共享分服配置也不会自动触发新的 generation。
- Agent 先回报 `deploying`，再执行本地 runner；runner 成功后保存本地 current / applied 状态并确认。失败不推进本地状态，回报 `deployment_failed` 等安全错误码，下个周期再尝试。
- 默认周期 1 分钟，可配置 10 秒至 1 小时；按 `server_id` 加稳定的 ±10% 抖动。

Linux runner 顺序为预拉取 → 备份（启用时）→ 校验 PID 所属实例并发送 SIGTERM → 等待正常退出 → 替换 → 启动与已提供的健康检查。停服默认等待 180 秒，超时中止换包；正常发布不启用强杀。`logs/`、`run/`、`data/` 与本地 `.env` 受保护。详情见[安全部署](SAFE_DEPLOY.md)。

Windows runner 当前使用预拉取 → `Stop-Process` → 备份 → 替换 → 启动与可选健康检查，超时路径含强制停止；不能按 Linux 的 SIGTERM / Flush 语义验收。需要持久化玩家数据的 Windows 服务，应先提供并验证符合业务要求的 runner，再启用自动部署。

## 自动规则与观察

启用的 Release Hook 在版本发布或显式 trigger 后进入尾随防抖，默认 30 秒，可设 1–3600 秒。等待期间的新事件延后执行时间并替换待部署版本；上传文件本身不触发。Hook 使用关联任务的目标范围与标签，版本来自最后的触发事件，而不是任务中固定的版本。Hook 队列持久化，中央服重启后继续处理。

开启自动跟随的 Pull 节点在版本发布后更新期望版本，由节点下次心跳执行；关闭自动跟随不会自动回滚已经设置的期望版本。分批或灰度上线需显式建立范围并逐批操作，当前标签不是自动波次调度器。

执行状态 `queued` / `running` / `success` / `failed` 分别表示排队、执行中、执行器完成和失败。查看逐服输出定位结果；长日志弹窗内部滚动，可用底部“关闭”或 Escape 退出查看，关闭不会取消部署。操作日志统一保留 30 天。

交付总览的按日上传、拉取和 SSH 执行统计是历史；节点在线、版本漂移、SSH 最近检查与 Hook 积压是刷新时刻状态，二者不能混用。服务器启动与脚本成功之外，仍需验证实际游戏登录和玩家数据链路。

## 接入与凭据

上传、发布、任务和期望状态写入使用有对应项目管理权限的 JWT / Basic 会话；Pull Token 不等于管理 JWT。SSH 凭据加密保存，API 不回传密码或私钥；主机指纹应显式核对。中央服生产访问使用 TLS，环境变量和 Secret 从运行环境注入，不写入版本包、仓库、VCS 元数据或日志。

公开指南目录为 `GET /api/agent/guide`，主题为 `GET /api/agent/guide/{topic}`，返回 JSON 内的 Markdown `content`。主题仅有 `overview`、`agent`、`gitea`、`github`、`security`；没有独立的 `/guide/` 文档路由。
