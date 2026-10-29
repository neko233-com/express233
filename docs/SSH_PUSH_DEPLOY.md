# SSH 批量推送部署

推送部署由管理 API 建立目标机和 `project|server_id` 唯一绑定，再按目标顺序串行执行。每台目标机必须预先安装 `express233-cli`、Bash、flock、GNU tar/coreutils、rsync 与 v2 `scripts/safe-deploy.sh`（置于 `PATH`）。脚本先拉取到独立暂存目录并计算指纹，取得逐目标锁、检查幂等状态后，才按绑定策略准备回滚包和发送 SIGTERM。旧进程正常退出后换包、启动并执行应用健康脚本。`express233-cli deploy` 与 `pull` 本身都是直接解压并执行 Hook，安全顺序由上述脚本提供。

缺少生命周期脚本会在停服前拒绝；启动或健康检查失败且应用允许恢复时，存在备份才恢复上一个版本。收到 HUP/INT/TERM 或 Hook 返回 75 时保留现场并阻断重试；SSH 断开须核验实际进程与日志。默认启用备份并保留最近 5 份（`EXPRESS233_BACKUP_KEEP` 可调）；绑定勾选“跳过旧版备份”后不会创建回滚包。rsync 换包及回滚保留实时 `data/`、`logs/`、`run/`、`.env`；`config/` 按发布配置规则合并或恢复，详见 [安全部署](SAFE_DEPLOY.md)。

部署创建时持久化完整计划摘要；排队后主机、实例路径、工件或配置改变会使整批执行在首台停服前失败。执行期间配置 API 返回 409，避免节点拉取到另一份配置。目标串行执行且首个失败后其余标记 skipped，SSH 命令派发后的连接丢失标记 unknown，不自动重试；重新启动控制器也不会重放 queued/running 历史任务。在途工件受保护，保留策略跳过该版本。

## 安全要求

SSH Push 会快照所选租户、所选 `server_id` 的 `post_hook_env`，通过 SSH 标准输入传入 `safe-deploy.sh` 及下游启动脚本。变量名必须是合法环境变量名，值按字面量引用，不展开命令；发布器固定的项目、服号、版本和目录优先于自定义同名变量。参数不写入进程参数或任务日志；自定义脚本仍须避免主动打印环境值。

- 控制台初始用户名是 `root`；本地开发初始密码是 `root`，生产首次登录后必须立即轮换。
- 登录密码使用 bcrypt；SSH 密码和私钥使用 `EXPRESS233_PUSH_CREDENTIAL_KEY`（base64 编码的 32 字节随机值）进行 AES-256-GCM 加密。MD5 不适合密码或凭据储存，不能使用。
- 新建 SSH 目标必须提供 OpenSSH `host_key`。仅在受控首次登记时可设置 `EXPRESS233_PUSH_ALLOW_TOFU=1`；已登记的指纹永不自动替换。
- HTTP 管理接口必须在 HTTPS 反向代理后使用。SSH 密码至少 8 字符，私钥会在保存前解析校验；`agent` 模式只适用于运行服务进程可访问 `SSH_AUTH_SOCK` 的环境。
- Git、YAML、示例和测试只保存账号名与 Secret 引用；由运行环境或未跟踪配置注入实际地址和凭据。当前主机 API 接受写入字段 `credential` / `private_key`，没有 Secret 引用解析字段；需要通过安全 HTTP 客户端在内存中解析 Secret 后发送，不能把密码展开到 curl 参数。加密主密钥应独立保管，不能在普通升级中重新生成。
- 登录保护默认同一 IP 在 15 分钟内失败 5 次后封禁 15 分钟；重复触发时按倍数延长，最长 24 小时。管理员可通过 `GET /api/security/login-ip-bans` 查看、`DELETE /api/security/login-ip-bans/{ip}` 解除。通过反向代理时必须设置 `EXPRESS233_TRUST_PROXY=1`，否则伪造的转发头不会被信任。
- 推送只接受 Linux SSH 目标；Windows 节点使用原有拉模式和 `.ps1` hook。上传包、拉取包均保留嵌套目录、二进制、`.sh` 与 `.ps1` 文件；标签会传给安全部署脚本。

## API

管理员在全局“机器与目标”页使用 `/api/push/hosts` 创建、维护 SSH 目标，并通过 `/api/push/hosts/{hostID}/servers` 绑定一个或多个 `project|server_id`；`project_name` 必填，未归属项目的历史绑定不会参与任何项目发布。项目“部署”步骤默认调用 `/api/projects/{id}/push-deployments` 并设置 `all_project_targets=true`，直接部署该项目的最新已发布版本与全部当前绑定，不依赖旧任务或标签。只有固定版本、固定 `server_ids` 或标签策略才需要 `/api/projects/{id}/push-tasks` 高级任务。每次执行都会将实际版本和筛选条件快照到发布日志。客户端应为一次执行生成 `Idempotency-Key`：网络重试复用原键会返回原执行，用户主动再次发布则生成新键。

远端同一内容指纹已记录、PID 存活且可选健康脚本通过时直接成功返回，不重启进程；未提供健康脚本不能视为业务已就绪。正式停服前还会校验 PID 可执行文件归属于目标目录。默认优雅退出等待 180 秒，每 200ms 检查一次；超时会在换包前中止，不发送 SIGKILL。仅人工明确授权应急强杀时，才可设置 `EXPRESS233_FORCE_KILL_AFTER_TIMEOUT=1`。

部署 API 的 `dry_run: true` 用于查看选中的版本和目标，保存带预览标记的执行记录，不建立 SSH 连接；它与目标机 `safe-deploy.sh --dry-run` 是不同层的预览，均不能证明真实服务可用。

凭据写入字段可在创建或更新目标时提供，所有读取接口都不会返回 SSH 密码或私钥。控制台不主动记录凭据，但自定义脚本必须避免输出 Secret；不要打印环境、启用 shell 跟踪或输出完整认证 URL。发布日志、逐目标输出、SSH 检查、上传、拉取、审计与安全记录均按 30 天滑动窗口清理；发布日志不可手工删除，删除任务定义不会影响历史执行快照。

### 版本发布自动 Hook

- 在项目“自动 Hook”页将一个 Hook 关联到同一项目的可重复发布任务。跨项目任务无法关联；目标也只从该项目绑定中解析。Hook 可随时启停；停用会取消仍在等待窗口内的触发，不需要反复删除和重建。
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
