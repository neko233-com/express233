# 部署指南

## Kubernetes（Helm）

```bash
helm install express233 ./deploy/helm/express233-server \
  --namespace express233 --create-namespace \
  --set image.tag=0.1.0 \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=express233.your.domain
```

数据持久化默认开启（PVC `/data`），内含 SQLite、`server.yaml` 与版本文件。

## Docker Compose

```bash
docker compose up -d
```

## Cloudflare Worker / Pages

Cloudflare 适合作为入口层，不适合作为当前 Go server 的运行时：

- Worker：代理 `express233-server` origin，提供统一 HTTPS 域名。
- Pages：可托管静态控制台，但 `/api/*` 仍需代理到 central server。
- Tunnel：适合 origin 在内网或家用机器时安全回源。

示例和限制见 [CLOUDFLARE.md](CLOUDFLARE.md)。

## 节点 CLI 配置

```bash
express233 config init
express233 config set server http://express233.your.domain:23380
express233 config set token <pull_token>
express233 config set project mygame
express233 config show
```

之后可简写：

```bash
express233 deploy --server-id game-logic-001 --dest /opt/game/001
```

## API 文档

服务启动后访问 `/docs/`（Swagger UI），或下载 `/api/openapi.yaml`。

## 监控与审计

- **指标**：`GET /metrics`（Prometheus 文本格式）
- Helm 开启：`serviceMonitor.enabled: true`（需集群有 Prometheus Operator）
- **审计**：管理员 Web「审计」页或 `GET /api/audit-logs`
- **备份**：`bash scripts/backup-data.sh [数据目录] [输出前缀]`

## 回滚与恢复

```bash
# 节点回滚到上一发布版本
express233 rollback --server-id game-logic-01 --dest /opt/game/01

# 指定版本
express233 rollback --to 1.0.0 --server-id game-logic-01

# 中央数据恢复
bash scripts/restore-data.sh ./backup.tar.gz /var/lib/express233
```

## 安全建议

1. 首次登录后立即 `Web → 账号` 修改 root 密码
2. 为每台部署机创建独立拉取账号或定期 `刷新 Token`
3. 不要将 token 提交到 git
