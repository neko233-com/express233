# README 截图

这些截图由无头 Agent Browser 在 Express233 v1.4.0 的本地隔离实例上生成，桌面尺寸为 **1440 × 900**。数据来自虚构项目 `game-demo`、区服 `1001` 至 `1003` 和保留测试网段里的机器绑定。部署预演仅解析目标，正式部署确认在截图后取消。

| 图片 | 内容 |
| --- | --- |
| `deployment-workflow.png` | 最新已发布版本、SSH 目标及预演结果 |
| `deployment-confirm.png` | 逐个核对本次部署目标、版本和备份策略 |
| `multi-version.png` | 两个已发布版本、一个草稿与文件浏览 |
| `server-config.png` | 按 server_id 编辑覆盖参数 |
| `template-replacement.png` | 原始配置和替换后的配置 |
| `file-tags.png` | 上传标签和批量处理 |
| `version-diff.png` | 已发布版本之间的配置差异 |
| `storage-space.png` | 容量、目录和关联删除范围 |

从仓库根目录运行以下命令，先把截图写入独立目录，检查可读性后再更新 README 图片。

```powershell
$env:EXPRESS233_BROWSER_CAPTURE = Join-Path $PWD '.tmp-readme-screenshots'
.\visual-e2e.cmd
```

已安装 Chrome 时可直接运行。需要下载时先执行 `npm --prefix test/browser ci` 和 `npm --prefix test/browser run install-browser`。可以设置 `EXPRESS233_BROWSER_SERVER`，用同一脚本验收已下载的正式发行二进制。
