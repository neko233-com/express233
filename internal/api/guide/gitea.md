# Gitea Actions 接入

推荐让 Gitea Actions 只负责构建和调用 express233 HTTP API：创建草稿版本、上传二进制/嵌套配置/`.sh`/`.ps1`、运行校验并发布。

发布成功后，已启用的 Release Hook 会进入尾随防抖窗口。默认 30 秒内的多次发布合并为一次任务，避免反复重启同一组游戏服。

将 API Token 存为 Gitea Secret；不要把账号密码、SSH 密钥、数据库地址或服务器列表写进 workflow、日志或版本包。使用版本号作为制品身份，不要以“覆盖最新版”代替多版本发布。
