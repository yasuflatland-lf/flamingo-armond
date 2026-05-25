# PWA (installable app + service worker)

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

## Scope and approach

The PWA surface delivers three things: an **installable** app (web app manifest + icons + iOS add-to-home-screen hint), **online card learning** (the existing learn flow is unchanged), and **service-worker shell resilience** with **automatic update propagation**. Offline *swipe learning* is explicitly out of scope — every swipe persists FSRS state through a GraphQL mutation, which needs the network; there is no local write queue.

Turbopack is kept. The service worker is **served as a static file from `public/`**, not bundled — so no Serwist / Workbox / webpack plugin is introduced. The source lives in `frontend/sw-template.js`; a pre-build Node script (`scripts/stamp-sw-version.mjs`) substitutes a version stamp and writes the generated `frontend/public/sw.js`. The stamp step runs outside the bundler, so the SW never participates in module resolution or HMR.

## Service worker fetch handling is auth-safe by construction

The `fetch` handler in `frontend/sw-template.js` applies four pass-through guards before it considers caching anything. Each guard exists to keep authenticated or mutating traffic away from the cache:

1. **Non-GET** (`request.method !== "GET"`) — mutations (POST/PUT/PATCH/DELETE) must reach the network untouched; caching or replaying a mutation is unsafe.
2. **Cross-origin** (`url.origin !== self.location.origin`) — Supabase auth/storage and any external origin must never be intercepted.
3. **`/api/graphql`** — auth-dependent and fetched with `cache: "no-store"`; caching it could serve one user's data to another.
4. **`/auth/`** (`url.pathname.startsWith("/auth/")`) — auth callbacks carry one-time codes and set session cookies; they must not be cached or short-circuited.

The load-bearing invariant is the **ordering of the next two rules: navigations are handled BEFORE the static-asset cache-first rule.** A top-level navigation is always a document load, and its URL may end in a static-looking suffix — for example a dynamic route segment such as `/learn/<id>.js`. If the extension-based static rule ran first, that navigation would match (`.js` is in the static extension list), be cached as authenticated HTML in the runtime cache, and then be replayed to a different user on a shared device. Checking `request.mode === "navigate"` first guarantees page HTML is never written to any cache.

Navigations are **network-only with an `/offline.html` fallback**. On a network failure the handler serves the precached offline page. Because the precache write can lose a first-install race, `caches.match("/offline.html")` may resolve to `undefined`; returning `undefined` to `respondWith()` throws and surfaces the browser's raw error page, so the fallback coalesces (`offline ?? new Response(...)`) into a synthetic `503` text response.

## Cache model

Two caches with deliberately different lifecycles:

- **`PRECACHE`** — name embeds the version stamp (`flamingo-precache-<stamp>`). Populated atomically at `install` via `cache.addAll`, holding `/offline.html`, `/icon-192.png`, `/icon-512.png`. A new version's `activate` sweeps away every cache whose name is neither the current `PRECACHE` nor `RUNTIME`, so the old precache is dropped automatically when the stamp changes.
- **`RUNTIME`** — fixed name (`flamingo-runtime`), cache-first. Holds `/_next/static/**` (content-hashed and therefore immutable: a given URL always returns the same bytes) plus stable same-origin assets matched by an extension list (`.png`, `.svg`, `.ico`, `.woff2`, `.css`, `.js`).

The accepted tradeoff of the fixed `RUNTIME` name: a **non-content-hashed** asset (e.g. an icon served from `public/` at a stable URL) can be retained stale across deploys, because the cache name never changes to force eviction. This is acceptable precisely because those same icons also live in the version-bumped `PRECACHE` and change rarely; the content-hashed `/_next/static/**` chunks are immune by construction.

## Update propagation without manual bumps

`/sw.js` is served with a no-cache directive (`Cache-Control: no-cache, no-store, must-revalidate`) via the `headers()` rule in [`next.config.ts`](../../frontend/next.config.ts). The reason is specific: the Service Worker **update algorithm** throttles its re-check of the worker script to **at most once per 24 hours** — a Service Worker spec rule, independent of any HTTP cache TTL. Without `no-cache`, a freshly deployed worker could go undetected for a day. With it, the browser revalidates `sw.js` on every page load and detects a new worker by byte comparison.

The build-time stamp (`scripts/stamp-sw-version.mjs`, Node built-ins only) computes the version as the first 8 hex chars of a SHA-256 digest over the hash inputs **in a fixed order**: the template itself (hashed with its `__SW_VERSION__` placeholder still in place, so identical inputs always yield the same version) plus the precached static files (`offline.html`, `icon-192.png`, `icon-512.png`). The precache version therefore bumps **only when the shell inputs change**. Deploys that touch app code only do not change the stamp — they propagate via network-only HTML plus content-hashed `/_next/static/**` chunks, so no manual version bump is needed.

The generated `frontend/public/sw.js` is **gitignored** (`.gitignore`) and **excluded from Biome lint** (`!public/sw.js` in [`biome.json`](../../frontend/biome.json)). It is regenerated by the `prebuild` and `predev` scripts in [`package.json`](../../frontend/package.json), so it always exists before `next build` / `next dev`.

## The web app manifest is a dynamic route, not a static file

`/manifest.webmanifest` is produced by `frontend/src/app/manifest.ts` (an App Router metadata route), not served from `public/`. It is therefore **deliberately absent from the atomic `cache.addAll` precache**: `addAll` is all-or-nothing, so a single failed fetch rejects the whole `install` and the worker never activates. A dynamically-rendered route is exactly the kind of fetch that can fail transiently, so including it would put the entire install at the mercy of one server-rendered response. It also cannot be content-hashed by the stamp script, which hashes static files on disk.

## Next 16 metadata placement

In `frontend/src/app/layout.tsx`, `appleWebApp` belongs in the `metadata` export, but `themeColor` MUST live in a **separate `viewport` export** (`export const viewport: Viewport = { themeColor }`), not inside `metadata`. Next 15+/16 moved `themeColor` (and other viewport fields) out of `Metadata` into `Viewport`; placing it in `metadata` is silently ignored. See the [`Metadata` / `Viewport` import-and-placement entry in `gotchas-encountered.md`](gotchas-encountered.md).

## Registration and the iOS install hint

`frontend/src/components/pwa/sw-register.tsx` registers `/sw.js` once after mount, guarded by both `"serviceWorker" in navigator` (skip unsupported browsers) and `window.isSecureContext` (skip plain HTTP, which would otherwise throw `SecurityError`). Registration failures are **warn-only** (`console.warn`), never thrown — a failed worker registration must not break the app.

iOS Safari has no `beforeinstallprompt` event, so installability cannot be surfaced through the standard prompt. `frontend/src/components/pwa/apple-install-hint.tsx` instead shows an add-to-home-screen hint. It is **hydration-safe**: it starts hidden (`useState(false)`) and only decides visibility in a post-mount `useEffect`, because the server cannot know the UA or dismissal state. The decision checks **both** standalone signals — the `(display-mode: standalone)` media query AND `navigator.standalone` — plus a `localStorage` dismissal flag, all wrapped in `try/catch` so a missing browser API never throws.

## Auth middleware excludes the static PWA endpoints

The Supabase auth middleware matcher in `frontend/src/middleware.ts` excludes `sw.js`, `offline.html`, and `manifest.webmanifest`. These are static PWA endpoints with no session to rotate, so running `getUser()` / cookie-rotation on them is wasted work — `sw.js` especially, since `no-cache` makes the browser re-fetch it on every page load. See [`auth-supabase.md` § "Middleware cookie rotation"](auth-supabase.md#middleware-cookie-rotation) for the full matcher rationale.

## Verification note

Unit and component tests cover the registration guard and the install-hint logic (`frontend/src/components/pwa/sw-register.test.tsx`, `frontend/src/components/pwa/apple-install-hint.test.tsx`). The service worker's *runtime* behaviour — offline-fallback rendering, install-time precache population, the actual install flow — cannot be exercised in the test environment because service workers require a secure context. Validating that path requires a real HTTPS device or a deployment.
