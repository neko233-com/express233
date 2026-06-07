# Cloudflare 部署适配

Cloudflare Workers / Pages 不能直接运行 `express233-server` Go 二进制，也没有可持久写入的本地磁盘。因此当前推荐形态是：

- `express233-server` 仍部署在能运行进程和持久化数据的平台：VPS、Docker、Kubernetes、Fly.io、Render、Railway、NAS 等。
- Cloudflare Worker 部署在边缘作为 HTTPS 入口、鉴权边界和反向代理。
- Cloudflare Pages 可选托管静态控制台，但 API 仍需代理到 `express233-server`。

如果后续要做“纯 Cloudflare serverless 中央服”，需要单独实现 D1（元数据）+ R2（版本包 / blob）+ KV（会话/缓存）存储适配；这不是把现有 Go 二进制直接上传到 Worker 就能完成的。

## 方案 A：Worker 反向代理（推荐）

适用于你已经有一个 origin 能跑 `express233-server`，但希望统一用 Cloudflare 域名访问。

目录：[examples/cloudflare/worker-proxy](../examples/cloudflare/worker-proxy)

```bash
cd examples/cloudflare/worker-proxy
npx wrangler secret put ORIGIN_BASE_URL
npx wrangler deploy
```

`ORIGIN_BASE_URL` 示例：

```text
https://origin-express233.example.com
```

建议：

- Worker 自定义域名绑定为 `https://express233.example.com`。
- Origin 只允许 Cloudflare 回源或加一层 Basic Auth / Tunnel。
- 不缓存 `/api/pull*`、`/api/login`、`/api/users*` 等带 token、Cookie 或敏感响应的接口。
- 版本包只有 MB 级也不要默认缓存，因为 URL 中可能包含 pull token；如果要缓存，应先改成短期签名 URL 或 header token。

## 方案 B：Pages 静态控制台 + Worker/API

Pages 可以托管 `internal/api/web` 的静态资源，但控制台使用相对 `/api/...` 请求，所以还需要让同域名的 `/api/*` 走 Worker 或反向代理。

示例 `_redirects`：

```text
/api/* https://express233.example.com/api/:splat 200
/docs/* https://express233.example.com/docs/:splat 200
```

更稳的做法是直接让 Worker 作为整个域名入口，同时代理静态页面和 API；这样 Cookie、JWT、Basic Auth、Swagger 都保持同源。

## 方案 C：Cloudflare Tunnel

如果你的 central server 在内网或家用机器上：

```bash
cloudflared tunnel --url http://127.0.0.1:23380
```

然后把 tunnel hostname 指向 `express233-server`。这不需要暴露公网端口，也比把 Go server 改造成 Worker 更符合当前架构。

## 不支持的部署方式

- 直接把 `express233-server` Go 二进制部署到 Worker / Pages。
- 依赖 Worker 本地磁盘保存 SQLite、`server.yaml` 或版本包。
- 在 Worker 中执行 `post_hook` 或启动游戏逻辑服进程。

节点侧仍使用 release 中的 Go 二进制：

```bash
express233 config set server https://express233.example.com
express233 config set token <pull_token>
express233 deploy --project mygame --server-id game-logic-001 --dest /opt/game/001
```
