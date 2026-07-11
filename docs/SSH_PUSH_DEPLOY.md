# SSH 批量推送部署

推送部署由管理 API 建立目标机和 `server_id` 绑定，再按标签顺序串行执行。每台目标机必须预先安装 `express233-cli` 与仓库中的 `scripts/safe-deploy.sh`（置于 `PATH`）；脚本会执行拉取到临时目录、停止旧进程、备份、保留 `logs/` 与 `run/`、替换、启动及可选健康检查的安全流程。缺少启动脚本、启动失败或健康检查失败都会自动恢复并启动上一个备份，默认保留最近 5 份（`EXPRESS233_BACKUP_KEEP` 可调）。

## 安全要求

- 控制台初始用户名是 `root`；本地开发初始密码是 `root`，生产首次登录后必须立即轮换。
- 登录密码使用 bcrypt；SSH 密码和私钥使用 `EXPRESS233_PUSH_CREDENTIAL_KEY`（base64 编码的 32 字节随机值）进行 AES-256-GCM 加密。MD5 不适合密码或凭据储存，不能使用。
- 新建 SSH 目标必须提供 OpenSSH `host_key`。仅在受控首次登记时可设置 `EXPRESS233_PUSH_ALLOW_TOFU=1`；已登记的指纹永不自动替换。
- HTTP 管理接口必须在 HTTPS 反向代理后使用。SSH 密码至少 8 字符，私钥会在保存前解析校验；`agent` 模式只适用于运行服务进程可访问 `SSH_AUTH_SOCK` 的环境。
- 登录保护默认同一 IP 在 15 分钟内失败 5 次后封禁 15 分钟；重复触发时按倍数延长，最长 24 小时。管理员可通过 `GET /api/security/login-ip-bans` 查看、`DELETE /api/security/login-ip-bans/{ip}` 解除。通过反向代理时必须设置 `EXPRESS233_TRUST_PROXY=1`，否则伪造的转发头不会被信任。
- 推送只接受 Linux SSH 目标；Windows 节点使用原有拉模式和 `.ps1` hook。上传包、拉取包均保留嵌套目录、二进制、`.sh` 与 `.ps1` 文件；标签会传给安全部署脚本。

## API

管理员使用 `/api/push/hosts` 创建/维护 SSH 目标，使用 `/api/push/hosts/{hostID}/servers` 绑定一个或多个 `server_id`，然后调用 `/api/projects/{id}/push-deployments`。请求可传 `server_ids`、`tags` 与 `tag_match` (`all` 或 `any`)；先使用 `dry_run: true` 检查选择结果。

凭据字段只接受一次，所有读取接口都不会返回 SSH 密码或私钥。部署日志会记录目标和命令输出，但不会记录凭据。

### SSH 存活检测

- 新建主机默认启用，每 `3600` 秒检查一次；`health_check_interval_seconds` 可在 `60` 到 `604800` 秒之间调整，也可用 `health_check_enabled: false` 关闭。
- 定时检查串行执行。每次只发起一次 TCP 连接和 SSH 握手，失败不重试；无论成功还是失败，都等待完整配置周期再执行下一次。
- `POST /api/push/hosts/{hostID}/check` 立即执行一次手动检查，`GET /api/push/hosts/{hostID}/checks?limit=50` 返回不可变历史。
- 主机列表只暴露 `last_check_status`、延迟、错误摘要和下次时间，不暴露密码、私钥或加密密文。
- Prometheus 指标为 `express233_ssh_check_total` 与 `express233_ssh_check_errors_total`。

已登录 Agent 可通过 `GET /api/agent/capabilities` 自发现发布、配置替换、拉取、SSH 与数据大盘操作；完整请求结构见 `/api/openapi.yaml`。
