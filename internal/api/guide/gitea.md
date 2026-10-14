# Gitea Actions 接入

Gitea Actions 负责构建、检查、上传并发布不可变的游戏版本包。由 express233 的任务 / Hook 执行 SSH 部署，或由 Pull Agent 执行本地部署；不要把发布接口成功等同于游戏服已上线。

## 凭据与项目

管理 API 使用有项目管理权限的账号通过 Basic Auth 或登录取得的 JWT。Pull Token 只用于拉取 / Agent，不是管理 Bearer Token。凭据从 Gitea Secret 注入环境，使用能从环境读取认证信息的 HTTP 客户端；不要放入命令参数、workflow 明文、VCS 字段、归档或日志。

项目先在控制台创建。下面的 {id} 是项目 ID，{ver} 必须取创建版本响应中的 version。实际地址从部署环境注入；文档示例只使用 https://control.example.invalid。

## API 顺序

1. 构建并完成业务测试，检查程序、嵌套配置及启动 / 健康检查脚本。
2. POST /api/projects/{id}/versions，提交 name 和可选 vcs。
3. POST /api/projects/{id}/versions/{ver}/files，以 multipart/form-data 上传 file；单文件可传 path，标签字段为 tags。ZIP、TAR、TAR.GZ / TGZ 自动解包。
4. 必要时通过 PUT /api/projects/{id}/versions/{ver}/file-tags 或 POST /api/projects/{id}/versions/{ver}/file-tags/batch 设置文件标签。
5. GET /api/projects/{id}/versions/{ver}/validate，读取 ok、errors、warnings。修复 errors，再决定是否接受共享配置引用产生的 warnings；不要只按 HTTP 200 判断检查通过。
6. POST /api/projects/{id}/versions/{ver}/publish，检查响应状态与版本对象，再观察需要的自动部署结果。

发布前检查不运行游戏测试、浏览器 E2E 或 SSH 连接。不同平台二进制使用 linux-amd64 等组合文件标签；文件标签按任一匹配，不能把 linux 与 amd64 当作同时满足的两个条件。

## 版本号与 VCS 溯源

创建请求示例：

```json
{
  "name": "1.4.3",
  "vcs": {
    "provider": "gitea",
    "repository": "https://git.example.invalid/team/game-logic",
    "ref": "refs/tags/v1.4.3",
    "commit": "0123456789abcdef0123456789abcdef01234567",
    "run_url": "https://git.example.invalid/team/game-logic/actions/runs/42",
    "run_id": "42"
  }
}
```

把当前 Gitea runner 实际提供的 ref、SHA、run ID 和运行 URL 映射到这些字段，不要把表达式字符串当作实际值上传。仓库与运行 URL 必须为不含认证信息的 HTTPS 地址；provider、repository、ref、commit 必须完整，commit 为 7–128 位十六进制。

Tag 为 v1.4.3 时，正式包名应归一为 1.4.3，vcs.ref 保留原始 tag。分支构建可发送空 name，由服务端事务内分配下一个三段版本的 patch；CI 必须保存并使用响应中的 version，不能自行猜下一个编号。重跑时先判断对应版本状态，不覆盖已发布内容；需要改变产物时创建新版本。

发布时生成文件内容清单 SHA-256，不等于压缩包 SHA-256；VCS 来源与文件随后不可变。共享 server_id 配置仍独立更新，未被该摘要冻结。

## 部署与自动规则

发布成功后，启用的 Release Hook 进入尾随防抖，默认最后一次触发后等待 30 秒；等待内多次发布合并为最后触发的版本。上传本身不触发。Hook 沿用任务的目标子集 / 标签，版本取触发事件，而不是任务固定版本。开启自动跟随的 Pull 节点会收到新的期望版本。

需要人工批准或灰度时，先关闭相关自动规则，显式选定小范围后再运行任务。POST /api/projects/{id}/push-tasks/{taskID}/run 可用 dry_run:true 只解析目标；不会连接 SSH。正式执行用不同幂等键，相同执行的网络重试复用原 Idempotency-Key，再查询部署记录直到结束。

固定版本被清理时应明确选择其他已发布版本；固定 server_id 子集仍受任务标签限制。观察逐服输出，而不只观察排队响应。相关概念见公开主题 GET /api/agent/guide/overview。
