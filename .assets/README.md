# README 截图

这些图片由 `test/visual/tests/readme-assets.spec.ts` 使用 Chromium 截取当前真实 Web UI，分辨率为 **1440 × 900**，未拼接或替换页面内容。截图基于 v1.2.1 工作流，生成于 2026-10-14。

全部数据均为独立本地测试实例中的虚构演示数据：项目 `game-demo`，区服 `1001`、`1002`、`1003`，机器 `game-node-a`、`game-node-b`。版本包中的程序和重启脚本只是文档夹具，不是可部署的游戏程序。页面中的 `root @ default` 是隔离测试实例自动建立的演示账号。

| 图片 | 内容 |
| --- | --- |
| `deployment-workflow.png` | 当前项目、最新已发布版本、3 个 SSH 目标及真实预演结果 |
| `deployment-confirm.png` | 正式部署前逐一核对版本、server_id、机器和备份策略；截图后取消 |
| `multi-version.png` | 2 个已发布版本、1 个草稿，最新版本与文件内容 |
| `server-config.png` | 以表单编辑分服参数，显示 3 个 server_id |
| `template-replacement.png` | 原始模板与 1001 分服覆盖后的真实配置对照 |
| `file-tags.png` | 草稿文件树和平台标签 |
| `version-diff.png` | 1.2.0 与 1.2.1 的真实配置差异 |
| `storage-space.png` | 本地存储统计、版本目录和文件详情 |

## 重新生成

前提：Go、Node.js 24 或更新版本，以及 Playwright Chromium 已安装。在仓库根目录用 PowerShell 执行：

```powershell
$captureStamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$captureRoot = Join-Path (Get-Location) ".tmp-readme-assets/$captureStamp"
$env:CI = '1'
$env:EXPRESS233_VISUAL_PORT = '39493'
$env:EXPRESS233_BASE_URL = 'http://127.0.0.1:39493'
$env:EXPRESS233_VISUAL_DATA_DIR = Join-Path $captureRoot 'data'
$env:PLAYWRIGHT_HTML_REPORT = Join-Path $captureRoot 'report'
Push-Location test/visual
try {
  npx playwright test tests/readme-assets.spec.ts --retries=0 --output="$captureRoot/results"
} finally {
  Pop-Location
}
```

`CI=1` 禁止复用已运行的服务；数据目录每次独立创建。测试自动启动本地 Go 服务并在结束后关闭。仅允许 loopback 地址，主机使用保留测试网段且关闭健康检查，只执行 `dry_run=true` 预演，正式部署确认框会取消，不连接 SSH、不调用外部 Hook。

脚本通过真实本地 API 准备数据，通过页面交互选择版本和配置。每张截图前校验对应内容、等待字体和 toast 消失。生成后仍需逐张检查可读性；这些 PNG 会覆盖本目录同名文件，报告与测试临时数据留在 Git 忽略的 `.tmp-readme-assets` 中。
