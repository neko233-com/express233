---
name: express233-visual-verify
description: 通过无头 Agent Browser 快速验证 Express233 的打包控制台、用户操作和响应式布局。
---

# Express233 无头浏览器验收

优先复用一个 Agent Browser Chrome 会话，使用 `batch --json --bail --omit-command` 批量执行动作。用可读取的页面状态等待异步操作完成。任务结束时关闭隔离会话。

```bash
bash visual-e2e.sh
# 或在 Windows 运行 visual-e2e.cmd
```

脚本构建前端后，以 `CGO_ENABLED=0` 构建并启动隔离的 Go 服务。测试只访问 loopback，包括登录、上传、分服预览、发布、深链、部署预演和取消、集群、存储、设置及移动端。设定 `EXPRESS233_BROWSER_SERVER` 可直接检验已编译或已下载的同平台服务端二进制。

依赖为 Node 24 LTS、`test/browser/package-lock.json` 锁定的 Agent Browser 和本机 Chrome。Chrome 缺失时运行 `npm --prefix test/browser run install-browser`。需要留存脱敏截图时设置 `EXPRESS233_BROWSER_CAPTURE` 指向输出目录。

该验证独立于 GitHub Release 六项门禁和游戏服的在线业务验收。
