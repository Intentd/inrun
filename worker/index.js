// Cloudflare Worker — get.inrun.dev
//
// Serves install.sh and install.sh.sha256 from GitHub raw.
// Edge-caches responses for CACHE_TTL seconds (default 300).
// SHA256 is computed on-the-fly from the fetched script content
// so no committed checksum file is required.
//
// Paths served:
//   /              → same as /install.sh (curl | bash friendly)
//   /install.sh    → script, text/plain
//   /install.sh.sha256 → "<hex>  install.sh\n", sha256sum -c compatible
//   *              → 404

const UPSTREAM =
  "https://raw.githubusercontent.com/intentd/inrun/refs/heads/main/install.sh";

const PLAIN_TEXT = "text/plain; charset=utf-8";

export default {
  async fetch(request, env, ctx) {
    if (request.method !== "GET" && request.method !== "HEAD") {
      return new Response("Method Not Allowed", { status: 405 });
    }

    const ttl = parseInt(env.CACHE_TTL ?? "300", 10);
    const path = new URL(request.url).pathname;

    if (path === "/" || path === "/install.sh") {
      return serveScript(ctx, ttl);
    }
    if (path === "/install.sh.sha256") {
      return serveChecksum(ctx, ttl);
    }
    return new Response("Not Found", { status: 404 });
  },
};

// ── Upstream fetch ────────────────────────────────────────────────────────────

async function fetchScript() {
  let upstream;
  try {
    upstream = await fetch(UPSTREAM);
  } catch (err) {
    return { err: `Upstream unreachable: ${err.message}`, status: 502 };
  }
  if (!upstream.ok) {
    const status = upstream.status === 404 ? 404 : 502;
    return { err: `Upstream returned ${upstream.status}`, status };
  }
  const text = await upstream.text();
  return { text };
}

// ── /install.sh ───────────────────────────────────────────────────────────────

async function serveScript(ctx, ttl) {
  const cache = caches.default;
  const key = new Request("https://get.inrun.dev/install.sh");

  const cached = await cache.match(key);
  if (cached) return cached;

  const { text, err, status } = await fetchScript();
  if (err) return new Response(err, { status });

  const res = new Response(text, { status: 200, headers: commonHeaders(ttl) });
  ctx.waitUntil(cache.put(key, res.clone()));
  return res;
}

// ── /install.sh.sha256 ────────────────────────────────────────────────────────

async function serveChecksum(ctx, ttl) {
  const cache = caches.default;
  const key = new Request("https://get.inrun.dev/install.sh.sha256");

  const cached = await cache.match(key);
  if (cached) return cached;

  const { text, err, status } = await fetchScript();
  if (err) return new Response(err, { status });

  const encoded = new TextEncoder().encode(text);
  const hashBuf = await crypto.subtle.digest("SHA-256", encoded);
  const hex = Array.from(new Uint8Array(hashBuf))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");

  // Two-space format required by sha256sum -c
  const body = `${hex}  install.sh\n`;
  const res = new Response(body, { status: 200, headers: commonHeaders(ttl) });
  ctx.waitUntil(cache.put(key, res.clone()));
  return res;
}

// ── Shared headers ────────────────────────────────────────────────────────────

function commonHeaders(ttl) {
  return {
    "Content-Type": PLAIN_TEXT,
    "Cache-Control": `public, max-age=${ttl}`,
    "X-Content-Type-Options": "nosniff",
  };
}
