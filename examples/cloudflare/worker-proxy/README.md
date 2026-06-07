# express233 Cloudflare Worker Proxy

This Worker puts Cloudflare in front of an existing `express233-server` origin.
It does not run the Go server inside Workers.

## Deploy

```bash
npm create cloudflare@latest -- --existing-script
npx wrangler secret put ORIGIN_BASE_URL
npx wrangler deploy
```

Use an origin URL without a trailing slash:

```text
https://origin-express233.example.com
```

## Notes

- Keep `/api/pull*` uncached because pull tokens may be in the query string.
- Bind a custom domain such as `express233.example.com` to the Worker.
- Protect the origin with Cloudflare Tunnel, firewall allowlists, or origin auth.
