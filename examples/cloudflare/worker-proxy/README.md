# Express233 Cloudflare Worker Proxy

This example proxies an existing `express233-server` origin. It does not run the Go server, database, or deployment hooks inside Workers.

## Deploy the existing example

Use Node.js 24 LTS and an existing Wrangler login or environment-injected Cloudflare credentials. Run from the repository root:

```bash
cd examples/cloudflare/worker-proxy
npx wrangler secret put ORIGIN_BASE_URL
npx wrangler deploy
```

Enter the origin URL interactively, for example `https://origin.example.invalid`. Keep the real value out of Git and command arguments. Do not include credentials in the URL. The existing `wrangler.toml` already points to `src/worker.js`; no project generator is needed.

## Behavior and limits

- The Worker forwards requests and authentication headers; it does not authenticate users itself or add an origin password.
- All `/api`, `/api/*`, and `/metrics*` requests bypass the configured cache and receive `Cache-Control: no-store`. Do not add public caching for authenticated downloads.
- `/` and `/docs/*` have a 300-second cache policy. Keep frontend resources aligned with the origin version when updating.
- Query strings are forwarded. Current CLI Token requests may contain credentials in the URL, so sanitize access/error logs and never share raw authenticated URLs.
- Configure the custom domain and origin access restrictions separately. The example does not block direct origin access.
- A Pages `_redirects` rule cannot proxy an external domain; use an actual Worker route for the API. See the [Cloudflare documentation](https://developers.cloudflare.com/pages/configuration/redirects/#proxying).

See the [deployment guide](../../../docs/CLOUDFLARE.md) for Pages, Tunnel, and end-to-end acceptance boundaries.