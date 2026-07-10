# SSH 批量推送部署

推送部署由管理 API 建立目标机和 `server_id` 绑定，再按标签顺序串行执行。每台目标机必须预先安装 `express233-cli` 与仓库中的 `scripts/safe-deploy.sh`（置于 `PATH`）；脚本会执行拉取到临时目录、停止旧进程、保留 `logs/` 与 `run/`、替换、启动的安全流程。

## 安全要求

- 控制台初始用户名是 `root`；本地开发初始密码是 `root`，生产首次登录后必须立即轮换。
- 登录密码使用 bcrypt；SSH 密码和私钥使用 `EXPRESS233_PUSH_CREDENTIAL_KEY`（base64 编码的 32 字节随机值）进行 AES-256-GCM 加密。MD5 不适合密码或凭据储存，不能使用。
- 新建 SSH 目标必须提供 OpenSSH `host_key`。仅在受控首次登记时可设置 `EXPRESS233_PUSH_ALLOW_TOFU=1`；已登记的指纹永不自动替换。
- HTTP 管理接口必须在 HTTPS 反向代理后使用。SSH 密码至少 8 字符，私钥会在保存前解析校验；`agent` 模式只适用于运行服务进程可访问 `SSH_AUTH_SOCK` 的环境。
- 推送只接受 Linux SSH 目标；Windows 节点使用原有拉模式和 `.ps1` hook。上传包、拉取包均保留嵌套目录、二进制、`.sh` 与 `.ps1` 文件；标签会传给安全部署脚本。

## API

管理员使用 `/api/push/hosts` 创建/维护 SSH 目标，使用 `/api/push/hosts/{hostID}/servers` 绑定一个或多个 `server_id`，然后调用 `/api/projects/{id}/push-deployments`。请求可传 `server_ids`、`tags` 与 `tag_match` (`all` 或 `any`)；先使用 `dry_run: true` 检查选择结果。

凭据字段只接受一次，所有读取接口都不会返回 SSH 密码或私钥。部署日志会记录目标和命令输出，但不会记录凭据。
