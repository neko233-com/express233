# GitHub Actions 接入

GitHub Actions 构建并发布游戏版本包；express233 负责记录版本、配置渲染、SSH 部署和 Pull 节点期望状态。上传发布不应携带目标机器的 SSH 密码、私钥或业务数据库连接串。

## 最小集成流程

1. 在 CI 中构建并测试程序、配置模板、启动与健康检查脚本。
2. POST /api/projects/{id}/versions，提交正式版本名 X.Y.Z 和 vcs 来源；创建响应的 version 是后续步骤的权威版本号。
3. POST /api/projects/{id}/versions/{ver}/files，使用 multipart/form-data 的 file 上传文件或 ZIP / TAR 包。单文件可传 path；tags 控制包内文件分发。
4. GET /api/projects/{id}/versions/{ver}/validate；明确检查 JSON 的 ok / errors / warnings，HTTP 200 不代表 ok 为 true。
5. POST /api/projects/{id}/versions/{ver}/publish，使版本可拉取并冻结包内内容。
6. 根据策略观察 Release Hook、SSH 执行结果或 Pull Agent 版本；发布成功和排队成功都不表示游戏服已经上线。

管理接口需要对应项目管理权限的 JWT / Basic 会话；Pull Token 不能作为管理 JWT。将最小权限凭据放入 GitHub Secrets，注入进程环境，由 HTTP 客户端读取；不要拼进命令行参数或回显请求头。

## 构建来源与版本号

vcs 字段映射：

| 字段 | 来源 |
|---|---|
| provider | github |
| repository | github.server_url 与 github.repository 组成的无凭据 HTTPS URL |
| ref | github.ref |
| commit | github.sha |
| run_id | github.run_id |
| run_url | 仓库 URL 加 /actions/runs/{run_id} |

提交实际展开的上下文值，不提交字面表达式。repository / run_url 不能含凭据；provider、repository、ref、commit 必须完整。

Tag 为 v1.4.3 时，版本 name 使用 1.4.3，vcs.ref 保留 refs/tags/v1.4.3。分支构建可传空 name，让服务端原子分配下一个 patch；使用响应中的 version，避免并发 runner 猜出相同编号。CI 重跑不能向已发布版本继续上传，也不能重写 VCS 来源。

发布时计算的是文件内容清单 SHA-256，不是压缩包摘要。文件、文件标签和 VCS 字段发布后只读；共享 server_id 配置并未随版本冻结，回滚仍需检查当前配置。

## 发布后的部署策略

常规 SSH 操作在控制台使用“部署最新版本”，按 published_at 倒序、同时间按记录 ID 倒序取版本，部署当前项目全部绑定目标，不取版本页当前选中的历史版本。

需要固定版本或区服子集时保存高级任务，实际目标还会应用任务标签过滤，默认 test。固定版本已清理时必须显式选择替代版本，不会自动变成最新。任务 /run API 支持 dry_run:true，仅解析目标，不连接 SSH；正式执行必须明确范围，相同操作重试复用 Idempotency-Key。

启用的 Release Hook 在版本发布后默认等待 30 秒，期间后续触发会延后执行并替换待部署版本。Hook 使用关联任务的目标范围，但部署触发事件版本，即使任务定义固定了旧版本。上传本身不触发 Hook。Pull 自动跟随也会在发布后推进期望版本；需要人工批准的流程应事先关闭相关自动规则。

预演与真实部署在控制台分别标记。读取部署状态及逐服输出，必要时滚动长日志；关闭日志不会取消执行。真实游戏登录与玩家数据保存仍需业务验收。

## express233 自身的 CI / Release

不要把本段与游戏项目的业务测试混淆。本仓库当前 CI 包含 Ubuntu、Windows、macOS 的 Go 测试、跨平台二进制构建和 Bash / PowerShell 语法检查；另有 golangci-lint。发布前要求这些检查在目标 commit 全部成功，Release 工作流会再检查，visual-e2e 不参与发布门禁。

express233 的构建产物包含 PROVENANCE.json 与 SHA256SUMS。升级控制台或 CLI 时验证对应官方发布产物；控制台升级不等于游戏服务版本已部署。
