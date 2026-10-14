# Cloudflare 入口适配

本仓库的 Cloudflare 示例是现有 `express233-server` 的入口代理。Go 服务、SQLite、租户配置与版本包仍在可持久化的主机或容器中运行；示例没有实现把中央服务搬到 Workers / Pages 的存储适配。

## Worker 反向代理

使用 [worker-proxy](../examples/cloudflare/worker-proxy) 中已有的 `wrangler.toml` 与 `src/worker.js`。本地使用 Node.js 24 LTS，Cloudflare 认证通过已有登录或环境 Secret 提供：

```bash
cd examples/cloudflare/worker-proxy
npx wrangler secret put ORIGIN_BASE_URL
npx wrangler deploy
```

`secret put` 交互输入中央服务回源地址，不把真实地址写入仓库或命令参数。地址格式示例为 `https://origin.example.invalid`，不含用户名、密码或 Token。自定义域名可使用 `express233.example.invalid` 这一占位形式，实际值在 Cloudflare 配置。

当前代理保留请求方法、查询参数、请求体及一般认证头，移除 hop-by-hop headers；它**没有独立鉴权逻辑**，也不会自动替回源生成 Basic Auth。若另加入口认证，必须核对浏览器 Cookie、CI Basic Auth 与节点拉取仍能通过，不能覆盖 Express233 的 `Authorization`。

缓存行为以示例源码为准：

- `/api`、`/api/*` 和 `/metrics*` 禁用边缘缓存，响应补充 `Cache-Control: no-store`。
- `/` 与 `/docs/*` 提供 300 秒缓存配置，其余路径关闭缓存。不能把它理解为所有静态资源永久缓存。
- 不对带账号权限的包下载启用公共缓存；改用 Header 也不能单独解决跨用户缓存隔离。
- CLI 当前 Token 拉取仍使用查询参数，代理、访问日志与错误记录必须过滤认证 URL；不要分享原始请求或日志。

Origin 的网络访问策略需单独配置。Worker 不会自动阻止绕过边缘直连 Origin；Secret 也不能替代访问控制。

## Pages 文档与同源 API

仓库的 Pages 发布入口是 `docs/` 文档站，包含 `index.html` 与 Markdown 文档。它不是中央控制台运行环境，也不会获得数据库和发布能力。

如果另行托管 `internal/api/web` 的静态控制台，仍须让同源 `/api/*` 进入中央服务，并保持静态资源和服务端版本一致。Pages `_redirects` 的 `200` 代理只支持站内相对路径，不能用外部 Origin URL 反向代理；仓库 `examples/cloudflare/pages/_redirects` 中的旧跨域示例不能直接用于该场景。可使用 Worker 作为整个域名入口，统一代理页面与 API。[Cloudflare Pages 代理限制](https://developers.cloudflare.com/pages/configuration/redirects/#proxying)

## Tunnel

本地测试可用 Quick Tunnel：

```bash
cloudflared tunnel --url http://127.0.0.1:23380
```

该命令生成临时地址，进程停止后失效，不是稳定的生产入口；正式环境应配置受管理的 Tunnel 与固定 hostname，并通过 Secret 注入连接凭据。[Cloudflare Quick Tunnels](https://developers.cloudflare.com/tunnel/get-started/quick-tunnels/)

## 节点部署与验收

节点仍运行官方 Release 的 Go CLI，并使用 [安全部署脚本](SAFE_DEPLOY.md) 控制游戏服。中央地址、账号或拉取 Token 由运行环境注入：

```bash
: "${EXPRESS233_SERVER:?inject the public central URL}"
: "${EXPRESS233_PROJECT:?set the project}"
bash scripts/safe-deploy.sh --server-id game-logic-01 --dry-run
```

`express233-cli deploy` 与 `pull` 直接解压，不能代替安全停服换包。Worker 不能执行目标机 Hook 或启动游戏服；这些工作仍由 SSH / 节点脚本完成。

入口上线后分别验证登录、项目列表、上传、预览、发布和节点下载，检查错误是否含凭据、API 是否被缓存，以及大包是否符合当前 Cloudflare 套餐限制。边缘页面可访问不等于节点发布或数据完整性已经验收。