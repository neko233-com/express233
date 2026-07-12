# express233 统一交付控制平面

## 平台定义

express233 的核心对象不是 SSH 主机，也不是压缩包，而是以下声明式关系：

> 项目版本制品 + server_id 配置实例 + 交付节点 + 期望版本 + 可审计的收敛过程。

这使同一套平台能够覆盖游戏网关、逻辑服、战斗服，也能覆盖 Spark 等大数据节点。业务差异不复制整套版本包，而是在交付时按 `server_id` 将数据库连接、端口、分区等模板值渲染进嵌套 YAML / JSON / properties。

## 推拉一体，而不是两套系统

| 维度 | Push | Pull |
|---|---|---|
| 控制入口 | 可重复发布任务 / Release Hook | 节点 Agent 心跳 |
| 网络方向 | 控制面主动 SSH 到节点 | 节点主动 HTTPS 到控制面 |
| 节点来源 | SSH Host + server_id Binding | 首次认证心跳自动注册 |
| 版本状态 | 最近成功发布版本 | current / desired + generation |
| 适合场景 | 网络可达、集中运维、小到中型节点组 | NAT 内、弹性节点、大规模集群 |
| 安全部署 | 远端 `safe-deploy.sh` | 本机 `safe-deploy.sh` / `.ps1` |

两种模式共享版本、配置渲染、发布校验、日志和 UI 节点清单。Push 不复制 SSH 密钥；Pull 不保存 Agent 密码。节点表只显示运维所需的拓扑、在线和版本状态。

## Pull Agent 协议

Agent 调用 `POST /api/agent/nodes/heartbeat`，使用 `X-Express233-Token`、HTTP Basic Auth 或 JWT。请求只包含节点元数据、current version、applied generation、状态和受限的机器错误码；禁止上传原始命令行输出。

控制面返回：

```json
{
  "desired": {
    "version": "4.3.0",
    "generation": 12,
    "needs_deploy": true
  }
}
```

收敛不变量：

1. 只有 desired version 变化时 generation 才增加；CI 重放同一发布是幂等的。
2. Agent 先回报 `deploying`，执行 staging → stop → backup → swap → start → health gate。
3. runner 成功后原子持久化本地状态，再回报 applied generation。
4. runner 失败时 current/applied 不推进，只回报安全错误码；不在同一周期立即重试。
5. 每个 server_id 使用稳定 ±10% 抖动，避免大集群同时心跳。

## 自动发布闭环

版本发布会同时驱动两类消费者：

- Push：启用的 Release Hook 进入持久化尾随防抖窗口。30 秒内多次上传/发布合并为一个最新版本任务。
- Pull：启用“自动跟随”的节点更新 desired version/generation，Agent 在下个心跳周期收敛。

Hook、发布执行、目标输出、Pull 和审计日志默认保留 30 天。Prometheus `/metrics` 暴露 Agent 心跳错误、drift 观测和期望状态变更计数。

控制台的数据大盘把“历史”和“当前”分开：ECharts 只呈现按日上传、拉取、SSH 发布和失败压力；Pull 在线/漂移、SSH 最近健康检查、Hook 启用/积压/失败则作为刷新时刻的拓扑快照。这样不会把瞬时离线误读为历史失败率。

## 游戏集群拓扑建议

推荐标签：

- `role:gateway`、`role:logic`、`role:battle`
- `env:production`、`env:staging`
- `region:<region>`、`shard:<shard>`、`canary`

配置仍以 `server_id` 为最终实例键。一个典型顺序是网关兼容性检查 → 少量逻辑服 canary → 逻辑服分批 → 战斗服分批。当前任务选择器可完成串行发布；更严格的生产变更可在其上增加发布波次、人工批准和错误率门禁。

## 生产边界

- 控制面部署在 TLS 反向代理后，限制管理 API 来源；Pull Agent 使用最小权限项目账号或可轮换 Token。
- SSH 凭据加密保存且 API 永不回传；主机密钥必须固定，禁止静默接受变化。
- 一台机器的不同 server_id 必须隔离最终目录、`.tmp`、日志、PID 和备份。
- 多控制面副本需要外部共享数据库和分布式任务租约；SQLite 适合单控制面实例。
- 高合规场景建议在版本包上增加签名/证明，在 runner 执行前验证，而不只依赖传输层 TLS 和摘要。
