# Pull Agent 接入

Agent 调用 `POST /api/agent/nodes/heartbeat` 获取 desired version 和 generation。认证方式为 `X-Express233-Token`、HTTP Basic Auth 或 JWT。

示例只使用占位符：

```bash
express233-cli agent --server https://control.example.invalid --project game-cluster --server-id logic-021 --token "$EXPRESS233_TOKEN" --root /opt/game-servers --interval 1m
```

Agent 在部署成功、启动脚本和健康检查通过后才确认 applied generation。失败只提交受限错误码；不要上传命令输出、密码、Token、SSH 密钥或数据库连接串。

Linux 使用 `safe-deploy.sh`，Windows 使用 `safe-deploy.ps1`。两者均执行 staging → stop → backup → swap → start → health gate。
