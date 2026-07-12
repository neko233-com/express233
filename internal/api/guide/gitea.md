# Gitea Actions 接入

推荐让 Gitea Actions 只负责构建和调用 express233 HTTP API：创建草稿版本、上传二进制/嵌套配置/`.sh`/`.ps1`、运行校验并发布。

发布成功后，已启用的 Release Hook 会进入尾随防抖窗口。默认 30 秒内的多次发布合并为一次任务，避免反复重启同一组游戏服。

将 API Token 存为 Gitea Secret；不要把账号密码、SSH 密钥、数据库地址或服务器列表写进 workflow、日志或版本包。使用版本号作为制品身份，不要以“覆盖最新版”代替多版本发布。
# Gitea Actions：把 VCS 来源和制品一起上传

创建版本时传入 `vcs`：`provider=gitea`、无凭据 HTTPS 仓库地址、`github.ref`、`github.sha`、运行 URL 与运行 ID。随后上传包、发布版本、触发可重复发布任务或 Hook。

Tag 构建应把 tag 作为 `name`。分支构建可将 `name` 留空；控制面在事务内生成下一个三段语义版本 patch，CI 必须使用响应中的 `version` 继续上传和发布，不能在 runner 内自行猜测下一个编号。

控制面会在发布时生成内容清单 SHA-256。不要在已发布版本上重新上传或覆盖 VCS 字段；用新版本号重新构建。
