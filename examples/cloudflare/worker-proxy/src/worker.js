const HOP_BY_HOP_HEADERS = new Set([
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
]);

export default {
  async fetch(request, env) {
    const originBase = normalizeOrigin(env.ORIGIN_BASE_URL);
    if (!originBase) {
      return new Response("ORIGIN_BASE_URL is not configured", { status: 500 });
    }

    const incomingURL = new URL(request.url);
    const upstreamURL = new URL(originBase);
    upstreamURL.pathname = joinPaths(upstreamURL.pathname, incomingURL.pathname);
    upstreamURL.search = incomingURL.search;

    const headers = new Headers(request.headers);
    for (const name of HOP_BY_HOP_HEADERS) {
      headers.delete(name);
    }
    headers.set("x-forwarded-host", incomingURL.host);
    headers.set("x-forwarded-proto", incomingURL.protocol.replace(":", ""));

    const upstreamRequest = new Request(upstreamURL, {
      method: request.method,
      headers,
      body: request.body,
      redirect: "manual",
    });

    const response = await fetch(upstreamRequest, {
      cf: cachePolicy(incomingURL),
    });
    const responseHeaders = new Headers(response.headers);
    for (const name of HOP_BY_HOP_HEADERS) {
      responseHeaders.delete(name);
    }
    responseHeaders.set("x-express233-proxy", "cloudflare-worker");

    if (isSensitivePath(incomingURL.pathname)) {
      responseHeaders.set("cache-control", "no-store");
    }

    return new Response(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers: responseHeaders,
    });
  },
};

function normalizeOrigin(value) {
  if (!value) {
    return "";
  }
  return value.replace(/\/+$/, "");
}

function joinPaths(basePath, requestPath) {
  const base = basePath.replace(/\/+$/, "");
  const path = requestPath.startsWith("/") ? requestPath : `/${requestPath}`;
  return `${base}${path}` || "/";
}

function cachePolicy(url) {
  if (isSensitivePath(url.pathname)) {
    return { cacheTtl: 0, cacheEverything: false };
  }
  if (url.pathname === "/" || url.pathname.startsWith("/docs/")) {
    return { cacheTtl: 300, cacheEverything: false };
  }
  return { cacheTtl: 0, cacheEverything: false };
}

function isSensitivePath(pathname) {
  return (
    pathname.startsWith("/api/") ||
    pathname === "/api" ||
    pathname.startsWith("/metrics")
  );
}
