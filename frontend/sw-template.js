// Service Worker source. This file is NOT bundled by Turbopack; it is served as
// a static file. `scripts/stamp-sw-version.mjs` reads this template, replaces
// every `__SW_VERSION__` with a deterministic content hash, and writes the
// result to `public/sw.js` before each dev/build. Do NOT edit `public/sw.js`
// directly — it is generated and gitignored.

const SW_VERSION = "__SW_VERSION__";
const PRECACHE = `flamingo-precache-${SW_VERSION}`;
const RUNTIME = "flamingo-runtime";
// `cache.addAll` is atomic: if any single URL fails to fetch, the whole install
// rejects and the precache is left empty. Every entry here is a real static file
// under `public/`, so a fetch only fails when the network itself is down — never
// because of a dynamically-rendered route. The web app manifest is a dynamic
// Next.js route (`src/app/manifest.ts`), so it is deliberately NOT precached.
const PRECACHE_URLS = ["/offline.html", "/icon-192.png", "/icon-512.png"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(PRECACHE);
      await cache.addAll(PRECACHE_URLS);
      // Activate this SW immediately rather than waiting for all tabs to close.
      await self.skipWaiting();
    })(),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const names = await caches.keys();
      // Drop every cache that is not the current precache or the runtime
      // cache. Because PRECACHE embeds SW_VERSION, a new version's activate
      // sweeps away the previous version's precache automatically.
      await Promise.all(
        names
          .filter((name) => name !== PRECACHE && name !== RUNTIME)
          .map((name) => caches.delete(name)),
      );
      // Take control of already-open clients so they use this SW without a reload.
      await self.clients.claim();
    })(),
  );
});

self.addEventListener("fetch", (event) => {
  const request = event.request;
  // Parse the URL once and reuse it for every guard below.
  const url = new URL(request.url);

  // 1. Never touch non-GET requests. Mutations (POST/PUT/PATCH/DELETE) must
  //    reach the network untouched — caching or replaying a mutation is unsafe.
  if (request.method !== "GET") return;

  // 2. Cross-origin requests pass through. Supabase (auth, storage) and any
  //    other external origin must not be cached or intercepted by this SW.
  if (url.origin !== self.location.origin) return;

  // 3. GraphQL is auth-dependent and is fetched with `cache: "no-store"`.
  //    Caching it could serve one user's data to another, so it always
  //    goes straight to the network.
  if (url.pathname === "/api/graphql") return;

  // 4. Auth callbacks / token exchange must never be cached or short-circuited;
  //    they carry one-time codes and set session cookies. Pass them through.
  if (url.pathname.startsWith("/auth/")) return;

  // 5. NAVIGATIONS — network-only with an offline fallback. Checked BEFORE the
  //    static-asset rule below: a top-level navigation is always a document load
  //    (never a subresource), and its URL may end in a static-like suffix (e.g. a
  //    dynamic route segment such as `/learn/abc.js`). Handling navigations first
  //    guarantees authenticated page HTML is NEVER written to a cache and replayed
  //    to another user. On a network failure we serve the precached `/offline.html`.
  if (request.mode === "navigate") {
    event.respondWith(
      (async () => {
        try {
          return await fetch(request);
        } catch {
          // The precache write may not have completed (first-install race), so
          // `caches.match` can resolve to undefined. Returning undefined to
          // respondWith() throws and surfaces the browser's raw error page, so
          // fall back to a minimal synthetic response instead.
          const offline = await caches.match("/offline.html");
          return (
            offline ??
            new Response("You are offline. Please reload when connected.", {
              status: 503,
              headers: { "Content-Type": "text/plain; charset=utf-8" },
            })
          );
        }
      })(),
    );
    return;
  }

  // 6. STATIC ASSETS — cache-first against RUNTIME. `/_next/static/` URLs are
  //    content-hashed by the bundler and therefore immutable: the same URL always
  //    returns the same bytes. The extension list (.png/.svg/.ico/.woff2/.css/.js)
  //    also catches `public/` assets (icons, fonts) served at stable, non-hashed
  //    URLs; those are static enough to serve from cache offline. Guards 1–5 above
  //    already diverted every non-GET, cross-origin, GraphQL, auth, and navigation
  //    request, so no authenticated content reaches this branch.
  const isStaticAsset =
    url.pathname.startsWith("/_next/static/") ||
    [".png", ".svg", ".ico", ".woff2", ".css", ".js"].some((ext) => url.pathname.endsWith(ext));
  if (isStaticAsset) {
    event.respondWith(
      (async () => {
        const cached = await caches.match(request);
        if (cached) return cached;
        const response = await fetch(request);
        if (response.ok) {
          const cache = await caches.open(RUNTIME);
          // Clone before returning — a Response body can be read only once.
          cache.put(request, response.clone());
        }
        return response;
      })(),
    );
    return;
  }

  // Anything else (e.g. non-navigate document subresources we do not classify
  // as static assets) passes through to the network untouched.
  return;
});
