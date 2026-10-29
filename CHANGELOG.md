# Changelog

## v1.3.0 — 2026-10-29

- 生产集群部署固定工件、目标、配置和 Hook 环境摘要，执行前验证完整计划；首个目标失败停止后续发布。
- 数据目录单写者进程锁、部署持久化抢占、停止排空与中断未知结果，不自动重放历史停服动作；在途工件禁止删除。
- 节点 runner v2 要求 rsync 和应用健康脚本，增加实例归属与路径检查、阶段记录、阻断回执和人工恢复确认；控制器拒绝旧 runner。
- SSH 输出并发保护与大小上限，断线后的未知结果单独记录；修复并发首次主机密钥登记不能拒绝冲突的问题。
- Helm 固定单控制器与 Recreate 更新；Go 三平台测试和 Linux 部署事务 race 检查覆盖上述边界。

## v1.2.3 — 2026-10-24

### 修复

- SSH Push 将所选服务器的 `post_hook_env` 传入安全部署脚本及下游 Hook，修复参数在 `--skip-hook` 分段流程中丢失的问题。
- 环境变量名校验、值字面量引用和发布上下文优先级保护；参数继续通过 SSH 标准输入传递，避免进入进程参数。
- 安全部署回滚归档排除实时运行目录，与换包时的数据保留范围保持一致。

## v1.2.2 — 2026-10-14

### 文档与截图

- README 按游戏服日常发布流程重写，加入当前控制台的版本、分服配置、预览、部署目标和确认弹窗截图；公开图片只使用本地演示数据。
- 增加统一文档目录，同步网页阅读入口、内置指南、API 说明及安装、SSH / Pull、CI、升级与数据保留文档。
- 明确预演不连接 SSH、共享配置不随版本冻结、固定服号与固定版本的执行边界。
- 纠正 CLI `pull` / `deploy` 与安全 runner 的区别、平台停服差异、Release 平台矩阵与统一 `SHA256SUMS` 校验方式。
- 本版本未修改游戏服部署实现或数据库结构；截图生成脚本同步当前交互。
- 修正文档站手机端横向溢出，并完善截图测试在完整套件中的文件定位和数据隔离。

## v1.2.1 — 2026-10-12

### 修复

- “保存并发布”保留固定 server_id 子集；失效的固定版本要求重新选择，不再静默改成最新版本。
- 最新已发布版本按发布时间确定；部署总览和确认弹窗逐项列出目标服号、机器及版本。
- 预演结果与真实部署结果分别展示，预演不再影响节点最近部署版本。
- 配置预览区分“原始配置 / 替换后配置”；长日志在弹窗内部滚动，关闭按钮保持可达。

## v1.2.0 — 2026-10-12

### 改进

- 游戏服工作区按版本包、分服配置、配置预览、发布版本、部署上线组织日常操作。
- 分服参数提供表单入口；高级配置、固定发布任务和历史记录按需展开。
- 完善上传、版本切换、发布检查、下一步引导与窄屏交互，并补齐相关浏览器验收。
- 修正发布验证夹具的跨平台兼容问题，恢复三平台 CI 发布门禁。

## 较早版本记录

以下为历史功能汇总，具体版本与产物见 [GitHub Releases](https://github.com/neko233-com/express233/releases)。

### Added
- SSH 服务器绑定新增可见的“跳过旧版备份”策略；默认仍保留回滚包，按目标显式跳过时不创建旧版归档
- `express233-server` 运维命令：`start` / `stop` / `restart` / `status` / `port` / `set-port` / `reload-config` / `backup-config` / `restore-config` / `reset-root-password`
- `express233-server update`：自更新到最新或指定 Release，并自动重启中央服
- `express233-server enable-autostart` / `disable-autostart` / `autostart-status`：跨平台原生开机自启动控制（Linux systemd、macOS launchd、Windows schtasks）
- **多租户**：`tenants` 表与 `userdata/<slug>/` 隔离目录；用户/项目/pull token 按租户隔离；`root` 可 `POST /api/tenants` 创建租户
- **嵌套配置覆盖**：`server.yaml` 中按 basename 写 YAML 子树（如 `mysql.url` / `mysql.password`），深度合并进版本包；仍兼容扁平 dotted 键
- 版本审批流：`draft/rejected` → `pending_review` → admin 发布；`POST .../submit-review`、`POST .../reject`
- RBAC 角色：`admin` / `operator` / `viewer`（viewer 只读 GET）
- 版本配置 diff：`GET /api/deploy/diff`、`GET /api/pull/diff`；CLI `express233-cli diff`
- 版本回滚 CLI：`express233-cli rollback`（上一发布版或 `--to`）
- 发布前检查 API / Web「发布前检查」
- 已发布版本原始包下载（Web / `GET .../download`）
- 系统状态 `GET /api/status`
- 数据恢复脚本 `scripts/restore-data.sh`
- 操作审计日志（Web 审计页 + `GET /api/audit-logs`）
- Prometheus 文本指标 `GET /metrics`
- 用户改密 `POST /api/me/password`、管理员重置 `PUT /api/users/{id}/password`
- CLI `express233-cli doctor` 环境自检
- 数据备份脚本 `scripts/backup-data.sh`
- 拉取/发布等关键操作写入审计

### Changed
- `server_id` 列表按字母排序
- CLI 二进制与 Release 资产改为 `express233-cli`，README 与安装脚本补齐 `express233-server` 的 PowerShell / shell 一键安装与运维示例
- 中央服日志改为 `slog` 文本日志 + 滚动文件；默认 `info` 级别且不再记录高频 2xx 请求日志
