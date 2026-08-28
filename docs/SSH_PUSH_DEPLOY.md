# SSH 批量推送部署

推送部署由管理 API 建立目标机和 `project|server_id` 唯一绑定，再按标签顺序串行执行。每台目标机必须预先安装 `express233-cli` 与仓库中的 `scripts/safe-deploy.sh`（置于 `PATH`）；脚本先拉取、校验内容指纹并准备回滚包，再取得逐目标锁，之后才发送 SIGTERM。旧进程正常退出后快速换包、启动并执行健康门禁。缺少启动脚本、启动失败、健康检查失败或 SSH 会话中断时，脚本会尝试恢复并启动上一个备份。默认保留最近 5 份（`EXPRESS233_BACKUP_KEEP` 可调）。

## 安全要求

- 控制台初始用户名是 `root`；本地开发初始密码是 `root`，生产首次登录后必须立即轮换。
- 登录密码使用 bcrypt；SSH 密码和私钥使用 `EXPRESS233_PUSH_CREDENTIAL_KEY`（base64 编码的 32 字节随机值）进行 AES-256-GCM 加密。MD5 不适合密码或凭据储存，不能使用。
- 新建 SSH 目标必须提供 OpenSSH `host_key`。仅在受控首次登记时可设置 `EXPRESS233_PUSH_ALLOW_TOFU=1`；已登记的指纹永不自动替换。
- HTTP 管理接口必须在 HTTPS 反向代理后使用。SSH 密码至少 8 字符，私钥会在保存前解析校验；`agent` 模式只适用于运行服务进程可访问 `SSH_AUTH_SOCK` 的环境。
- 登录保护默认同一 IP 在 15 分钟内失败 5 次后封禁 15 分钟；重复触发时按倍数延长，最长 24 小时。管理员可通过 `GET /api/security/login-ip-bans` 查看、`DELETE /api/security/login-ip-bans/{ip}` 解除。通过反向代理时必须设置 `EXPRESS233_TRUST_PROXY=1`，否则伪造的转发头不会被信任。
- 推送只接受 Linux SSH 目标；Windows 节点使用原有拉模式和 `.ps1` hook。上传包、拉取包均保留嵌套目录、二进制、`.sh` 与 `.ps1` 文件；标签会传给安全部署脚本。

## API

管理员在全局 Agent / SSH 资源页使用 `/api/push/hosts` 创建、维护 SSH 目标，并通过 `/api/push/hosts/{hostID}/servers` 绑定一个或多个 `project|server_id`。项目发布页不再管理机器，而是通过 `/api/projects/{id}/push-tasks` 保存版本策略、`server_ids`、`tags` 与 `tag_match` (`all` 或 `any`)。调用 `/api/projects/{id}/push-tasks/{taskID}/run` 可重复预演或正式执行；每次执行都会将任务名称、实际版本和筛选条件快照到发布日志。客户端应为一次执行生成 `Idempotency-Key`：网络重试复用原键会返回原执行，用户主动再次发布则生成新键。

远端同一内容指纹已经运行且健康时直接成功返回，不重启进程。默认优雅退出等待 180 秒，每 200ms 检查一次；超时会在换包前中止，不发送 SIGKILL。仅在明确设置 `EXPRESS233_FORCE_KILL_AFTER_TIMEOUT=1` 时允许强杀。

凭据字段只接受一次，所有读取接口都不会返回 SSH 密码或私钥。部署日志会记录目标和命令输出，但不会记录凭据。发布日志、逐目标输出、SSH 检查、上传、拉取、审计与安全记录均按 30 天滑动窗口清理；发布日志不可手工删除，删除任务定义不会影响历史执行快照。

### 版本发布自动 Hook

- 在项目“自动 Hook”页将一个 Hook 关联到可重复发布任务。Hook 可随时启停；停用会取消仍在等待窗口内的触发，不需要反复删除和重建。
- `POST /api/projects/{id}/versions/{ver}/publish` 成功后会自动触发所有已启用 Hook。发布接口是幂等的：CI 对已发布版本重试会作为新的 Hook 触发进入同一合并窗口。
- 默认使用 30 秒尾随防抖。窗口内的新触发会刷新 `due_at` 并覆盖为最新触发版本；最终只创建一次 SSH 发布任务，批量目标仍按发布任务定义串行安全重启。
- 等待队列存储在 SQLite，中央服务重启不会丢失。每次派发使用 `hook_event_id` 作为唯一幂等键；超过租约时间的 `running` 事件会恢复，并复用已经创建的发布记录。多个到期 Hook 全局串行执行，避免两个任务同时重启重叠节点。也可调用 `POST /api/projects/{id}/release-hooks/{hookID}/trigger` 手动或由 Gitea/GitHub CI 显式触发。
- Hook 触发、合并、派发和失败均保存 30 天，但不会保存 HTTP 密码、Token、SSH 凭据或上传包内容。Prometheus 提供 `express233_release_hook_triggers_total`、`express233_release_hook_merges_total`、`express233_release_hook_dispatches_total` 和 `express233_release_hook_failures_total`。

### SSH 存活检测

- 新建主机默认启用，每 `3600` 秒检查一次；`health_check_interval_seconds` 可在 `60` 到 `604800` 秒之间调整，也可用 `health_check_enabled: false` 关闭。
- 定时检查串行执行。每次只发起一次 TCP 连接和 SSH 握手，失败不重试；无论成功还是失败，都等待完整配置周期再执行下一次。
- `POST /api/push/hosts/{hostID}/check` 立即执行一次手动检查，`GET /api/push/hosts/{hostID}/checks?limit=50` 返回不可变历史。
- 主机列表只暴露 `last_check_status`、延迟、错误摘要和下次时间，不暴露密码、私钥或加密密文。
- Prometheus 指标为 `express233_ssh_check_total` 与 `express233_ssh_check_errors_total`。

已登录 Agent 可通过 `GET /api/agent/capabilities` 自发现发布、配置替换、拉取、SSH 与数据大盘操作；完整请求结构见 `/api/openapi.yaml`。

用户仓库如何接收幂等键、执行 ID 与 `project|serverId`，以及如何实现最短停机，见 [发布脚本契约](DEPLOYMENT_SCRIPT_CONTRACT.md)。
