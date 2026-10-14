# Pull Agent 接入

Pull Agent 在游戏节点主动请求中央服，适合中央服无法主动 SSH 到达的网络。它和控制台的 SSH 部署是两种执行方式，不会被“部署最新版本”按钮一并部署。

## 接入前准备

1. 创建项目，上传并发布 X.Y.Z 格式的版本包。
2. 注册共享 server_id 配置；没有差异时 replacements 可以为空。配置按租户共享，不会随版本包冻结。
3. 为 Agent 准备有该项目访问权限的 Pull Token，或账号密码，通过 Secret / 进程环境注入 EXPRESS233_TOKEN，或 EXPRESS233_USERNAME / EXPRESS233_PASSWORD。不要把凭据写入命令行、仓库或输出。
4. 在节点安装 express233-cli 与经过验证的 runner，准备独立 GAME_ROOT/{server_id}、.tmp/{server_id}、logs、run 和备份路径。

Linux 启动示例，运行环境已注入凭据：

```bash
express233-cli agent \
  --server https://control.example.invalid \
  --project game-logic \
  --server-id logic-021 \
  --root /opt/game-servers \
  --runner /opt/express233/scripts/safe-deploy.sh \
  --interval 1m
```

runner 需要执行权限，express233-cli 需要在 PATH 中。默认周期 1 分钟，可设置 10 秒至 1 小时，并带按 server_id 稳定的 ±10% 抖动。--once 执行一个完整周期，可能真实部署，不是预演。

## 设置期望版本

Agent 心跳注册节点后，在控制台或管理 API 设置期望版本：

```http
PUT /api/projects/{id}/delivery-nodes/{serverID}/desired
Content-Type: application/json

{"version":"1.4.3","auto_follow":false}
```

此接口需要项目管理权限的 JWT / Basic 会话；Pull Token 不等于管理 JWT。指定版本必须已发布。设置 auto_follow 为 true 且省略 version 时选择当前最新已发布版本；后续发布继续推进。只变更共享分服配置不会自动增加 generation。固定版本已被清理时，应明确选择仍存在的版本。

## 心跳与执行结果

POST /api/agent/nodes/heartbeat 支持 X-Express233-Token、HTTP Basic Auth 或 JWT；CLI 使用 Token 请求头或 Basic Auth。请求包含项目、server_id、节点元数据、current_version、applied_generation、状态和受限 error_code，返回 desired.version、desired.generation、desired.needs_deploy。

Agent 先回报 deploying，再执行 runner。runner 成功后才保存本地版本和 generation 并回报；失败仅回报 deployment_failed 或 state_persist_failed 等安全错误码，本地 current / applied 不推进，下个周期再试。不要通过 error_code 上报原始命令输出、密码、私钥或数据库连接串。

Linux safe-deploy.sh 先拉到 staging，再按启用的备份策略备份；随后校验 PID 所属实例、发送 SIGTERM 并等待正常退出，再替换和启动，运行包内已提供的健康检查。默认停服超时 180 秒，超时中止换包。正常发布不启用强杀。Agent 调用 runner 时默认请求备份。

Windows safe-deploy.ps1 当前采用 Stop-Process，顺序为 staging → stop → backup → swap → start / 可选健康检查，超时路径含强制停止。它不提供与 Linux SIGTERM 相同的玩家数据 Flush 保证；有持久化要求的 Windows 游戏服需要先适配并验证 runner，不能只按脚本语法通过判断可上线。

## 单次拉取不等于安全换包

express233-cli pull 和 deploy 是同一入口：下载、解压到 --dest，默认执行已配置的 post_hook；--skip-hook 才会跳过。它们不会自行执行完整停服保护，不应直接覆盖运行中的二进制目录。

单次 /api/pull 也可能出现在节点清单，表示服务端交付了包，不证明节点启动成功。查看 Agent current / desired / applied、逐服结果并验证实际游戏链路；预演仅目标解析，不能证明这些步骤已完成。
