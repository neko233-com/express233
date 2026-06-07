# Changelog

## Unreleased

### Added
- `express233-server` 运维命令：`start` / `stop` / `restart` / `status` / `port` / `set-port` / `reload-config` / `backup-config` / `restore-config` / `reset-root-password`
- **多租户**：`tenants` 表 + 数据目录 `data/tenants/<slug>/`；用户/项目/pull token 按租户隔离；`root` 可 `POST /api/tenants` 创建租户
- **嵌套配置覆盖**：`server.yaml` 中按 basename 写 YAML 子树（如 `mysql.url` / `mysql.password`），深度合并进版本包；仍兼容扁平 dotted 键
- 版本审批流：`draft/rejected` → `pending_review` → admin 发布；`POST .../submit-review`、`POST .../reject`
- RBAC 角色：`admin` / `operator` / `viewer`（viewer 只读 GET）
- 版本配置 diff：`GET /api/deploy/diff`、`GET /api/pull/diff`；CLI `express233 diff`
- 版本回滚 CLI：`express233 rollback`（上一发布版或 `--to`）
- 发布前检查 API / Web「发布前检查」
- 已发布版本原始包下载（Web / `GET .../download`）
- 系统状态 `GET /api/status`
- 数据恢复脚本 `scripts/restore-data.sh`
- 操作审计日志（Web 审计页 + `GET /api/audit-logs`）
- Prometheus 文本指标 `GET /metrics`
- 用户改密 `POST /api/me/password`、管理员重置 `PUT /api/users/{id}/password`
- CLI `express233 doctor` 环境自检
- 数据备份脚本 `scripts/backup-data.sh`
- 拉取/发布等关键操作写入审计

### Changed
- `server_id` 列表按字母排序
- CLI 二进制与 Release 资产改为 `express233-cli`，README 与安装脚本补齐 `express233-server` 的 PowerShell / shell 一键安装与运维示例
