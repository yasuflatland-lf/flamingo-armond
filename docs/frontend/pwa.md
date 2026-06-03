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

The generated `frontend/public/sw.js` is **gitignored** (`.gitignore`) and **excluded from Biome lint** (`!public/sw.js` in [`biome.json`](../../frontend/biome.json)). It is regenerated by the `prebuild` and `predev` scripts in [`package.json`](../../frontend/package.json), so it always exists before `next build` / `next dev`. The template `frontend/sw-template.js` is read via `readFileSync` (never imported), so it is added to Knip's `ignore` ([`knip.jsonc`](../../frontend/knip.jsonc)) — otherwise Knip's module-graph scan reports it as an unused file and fails the CI lint job.

## The web app manifest is a dynamic route, not a static file

`/manifest.webmanifest` is produced by `frontend/src/app/manifest.ts` (an App Router metadata route), not served from `public/`. It is therefore **deliberately absent from the atomic `cache.addAll` precache**: `addAll` is all-or-nothing, so a single failed fetch rejects the whole `install` and the worker never activates. A dynamically-rendered route is exactly the kind of fetch that can fail transiently, so including it would put the entire install at the mercy of one server-rendered response. It also cannot be content-hashed by the stamp script, which hashes static files on disk.

## Next 16 metadata placement

In `frontend/src/app/layout.tsx`, `appleWebApp` belongs in the `metadata` export, but `themeColor` MUST live in a **separate `viewport` export** (`export const viewport: Viewport = { themeColor }`), not inside `metadata`. Next 15+/16 moved `themeColor` (and other viewport fields) out of `Metadata` into `Viewport`; placing it in `metadata` is silently ignored. See the [`Metadata` / `Viewport` import-and-placement entry in `gotchas-encountered.md`](gotchas-encountered.md).

## Registration and the iOS install hint

`frontend/src/components/pwa/sw-register.tsx` runs once after mount and applies three guards in order:

1. **Unsupported browser** (`"serviceWorker" not in navigator`) → no-op.
2. **Non-production** (`process.env.NODE_ENV !== "production"`, i.e. `next dev`) → does **not** register; instead calls `navigator.serviceWorker.getRegistrations()` and unregisters every existing worker so the browser self-heals. Failures are `console.warn("[pwa] service worker cleanup failed:", ...)`, never thrown.
3. **Production + secure context** (`window.isSecureContext`) → registers `/sw.js`. Failures are `console.warn(...)`, never thrown — a failed worker registration must not break the app.

**Why production-only?** The cache-first static-asset rule in `sw-template.js` assumes `/_next/static/` URLs are content-hashed and therefore immutable. That holds for `next build` output but not for `next dev`: Turbopack reuses stable dev chunk URLs for HMR, so a service worker in dev serves a stale client chunk and causes React hydration mismatches. The SW provides no dev value (its runtime behavior cannot be tested in the standard dev or CI environment anyway — that requires a real HTTPS deployment), so registration is intentionally skipped in development.

iOS Safari has no `beforeinstallprompt` event, so installability cannot be surfaced through the standard prompt. `frontend/src/components/pwa/apple-install-hint.tsx` instead shows an add-to-home-screen hint. It is **hydration-safe**: it starts hidden (`useState(false)`) and only decides visibility in a post-mount `useEffect`, because the server cannot know the UA or dismissal state. The decision checks **both** standalone signals — the `(display-mode: standalone)` media query AND `navigator.standalone` — plus a `localStorage` dismissal flag, all wrapped in `try/catch` so a missing browser API never throws.

## Auth middleware excludes the static PWA endpoints

The Supabase auth middleware matcher in `frontend/src/middleware.ts` excludes `sw.js`, `offline.html`, and `manifest.webmanifest`. These are static PWA endpoints with no session to rotate, so running `getUser()` / cookie-rotation on them is wasted work — `sw.js` especially, since `no-cache` makes the browser re-fetch it on every page load. See [`auth-supabase.md` § "Middleware cookie rotation"](auth-supabase.md#middleware-cookie-rotation) for the full matcher rationale.

## Verification note

Unit and component tests cover the registration guard, the production gate, and the dev-cleanup path (`frontend/src/components/pwa/sw-register.test.tsx`), as well as the install-hint logic (`frontend/src/components/pwa/apple-install-hint.test.tsx`). The service worker's *runtime* behaviour — offline-fallback rendering, install-time precache population, the actual install flow — cannot be exercised in the test environment because service workers require a secure context. Validating that path requires a real HTTPS device or a deployment.

## PWA black-flash on dark-mode iOS — `color-scheme: light`

### Root cause

The app currently renders light-only — no `ThemeProvider` or `prefers-color-scheme` toggle is wired, so the dormant `.dark` block in `globals.css` is never activated and `--background` stays `oklch(1 0 0)`. But the document declared no `color-scheme`. On an iOS standalone PWA whose device is in **dark mode**, Safari applies the system dark appearance to the UA *canvas* — the backdrop painted before `<body>`'s `bg-background` white reaches the screen. The result is a black flash on every launch, even though every painted surface is light.

### Fix

Declare the scheme as light in two places, both correct for a light-only app:

- `frontend/src/app/layout.tsx` — sets `colorScheme: "light"` in the `viewport` export, which emits `<meta name="color-scheme" content="light">`. The meta is parsed in `<head>` before first paint, so it governs the pre-paint canvas appearance — the exact interval where the black flash occurs.
- `frontend/src/app/globals.css` — `:root { color-scheme: light; }`, the CSS-level declaration that keeps the canvas, scrollbars, and form controls light once styles apply.

This is correct precisely because the app is light-only; it never wants the dark canvas. `frontend/src/app/layout.test.tsx` pins `viewport.colorScheme === "light"` so the meta cannot silently regress.

### Why not the apple-touch-startup-image

A mismatched `apple-touch-startup-image` produces a *sustained* black launch screen, not a brief flash. The observed symptom was a brief black flash that resolved into the app — the canvas-appearance issue above, not startup-image coverage. The startup images remain a separate concern (see the Interval A section).

## PWA white-screen fix: de-blocking the root layout (Interval B)

### Root cause

`frontend/src/app/layout.tsx` was an `async` server component that awaited the full Supabase auth waterfall (`getUser()` → `getClaims()`) before returning any JSX. During that await the server streamed zero bytes. The browser's body background (`oklch(1 0 0)`, pure white from `globals.css`) was visible for hundreds of milliseconds on cold start and on every service-worker-update reload. Auth identity is now resolved by middleware and forwarded via request headers, so the layout is synchronous — but the historical root cause is preserved here because the fix follows directly from it.

Adding `app/loading.tsx` alone does **not** fix this. `loading.tsx` wraps the `page` inside the layout in a Suspense boundary, but the Suspense fallback cannot render until the layout itself returns its first JSX chunk. The blocking await is in the layout, so the fallback is never streamed until after the delay has already occurred.

### Fix: synchronous `<html><body>` + middleware-forwarded auth identity

The layout now returns `<html><body>` synchronously. Auth identity (`shellUser`, `isAdmin`) is resolved by the middleware and forwarded to the RSC render via request headers, so the layout reads it from a fast in-memory map that Next.js pre-populates before the component runs — no async Supabase await remains in the layout function body. The `/login` and `/onboarding` fast-path branches (bare shell, no auth, no nav) retain their own early return.

For all other routes the layout reads the forwarded headers via `readAuthContext` and passes the resolved values synchronously into `AuthShell`:

```tsx
// frontend/src/app/layout.tsx (simplified)
const headersList = await headers();
const { shellUser, isAdmin } = readAuthContext(headersList);

// ...
<body suppressHydrationWarning>
  <Providers>
    <AuthShell user={shellUser} isAdmin={isAdmin}>
      {children}
    </AuthShell>
  </Providers>
  <SpeedInsights />
  <SwRegister />
  <AppleInstallHint />
</body>
```

The layout returns `<html><body>` synchronously because auth state is read from request headers set by middleware before the RSC render; no streaming-blocking await remains in the layout. The page skeleton streams immediately on every navigation.

### AuthShell degradation contract

`AuthShell` is now a synchronous prop-driven wrapper — it no longer performs any auth I/O. The degradation contract has moved to the middleware (`frontend/src/middleware.ts`): `getClaims()` is wrapped in a `try/catch` that fails closed to `isAdmin = false` on any exception, including non-`AuthError` throws from `validateExp` (plain `Error`) and WebCrypto (`DOMException`). The catch branch logs `err.name` only (no `err.message`, which may carry user-supplied content) and proceeds with the degraded identity. Because the middleware runs before the RSC tree, `AuthShell` always receives a fully-resolved `user` and `isAdmin` prop and never needs to handle an error path.

### Log prefix continuity

Middleware logs the identity-resolution failure with the `[middleware]` scope prefix. `AuthShell` retains the `[layout]` prefix for any layout-level structural errors. Operator runbooks and test assertions pin to those strings; they are not interchangeable.

### N5 — collapsing two loading states into one

Two stacked Suspense boundaries reveal sequentially: auth (a fast local check) resolves before the page data batch, so the previous implementation showed brand splash → page skeleton → page content on each navigation. Resolving identity entirely out of the render path (in middleware, before the RSC tree runs) collapses the first loading state, leaving exactly one loading state per navigation: the page skeleton. PWA cold start still shows the native launch screen from `layout.tsx` `metadata.appleWebApp.startupImage` (see the Interval A section below) — that interval is handled by the OS before the RSC tree runs at all and is unaffected by this change.

## iOS apple-touch-startup-image (Interval A)

### Root cause

iOS does not use the web app manifest's `background_color` for the standalone launch splash. Between the moment the user taps the home-screen icon and the moment WebView renders the first pixel, the OS shows a white screen. This is a distinct interval from the app-level delay described in the section above — it occurs before the app code runs at all, entirely within the OS.

### Fix: `apple-touch-startup-image` meta tags via Next.js `metadata`

The `metadata` export in `frontend/src/app/layout.tsx` declares 14 portrait-orientation splash PNGs via `appleWebApp.startupImage`. Each entry maps a CSS media query to a PNG file served from `frontend/public/splash/`. iOS Safari selects the entry whose media query matches the device's logical dimensions and device pixel ratio, then uses that PNG as the startup image.

**Physical pixel naming:** the file naming convention is `splash-<physW>x<physH>.png`, where physical pixels equal logical pixels multiplied by the device pixel ratio. The media query uses logical pixels and DPR separately so Safari can match correctly.

**Covered devices (portrait only; landscape is skipped — the app is portrait-oriented):**

| Device | Logical WxH | DPR | Physical file |
|---|---|---|---|
| iPhone 16 Pro Max | 440×956 | 3 | `splash-1320x2868.png` |
| iPhone 16 Pro | 402×874 | 3 | `splash-1206x2622.png` |
| iPhone 16 Plus / 15 Plus | 430×932 | 3 | `splash-1290x2796.png` |
| iPhone 16 / 15 / 14 Pro | 393×852 | 3 | `splash-1179x2556.png` |
| iPhone 14 / 13 / 12 | 390×844 | 3 | `splash-1170x2532.png` |
| iPhone 14 Plus / 13 Pro Max | 428×926 | 3 | `splash-1284x2778.png` |
| iPhone 11 Pro Max / XS Max | 414×896 | 3 | `splash-1242x2688.png` |
| iPhone 11 / XR | 414×896 | 2 | `splash-828x1792.png` |
| iPhone SE 3rd gen | 375×667 | 2 | `splash-750x1334.png` |
| iPad Pro 12.9" | 1024×1366 | 2 | `splash-2048x2732.png` |
| iPad Pro 11" / Air 4–5 | 834×1194 | 2 | `splash-1668x2388.png` |
| iPad Air 3 / Pro 10.5" | 834×1112 | 2 | `splash-1668x2224.png` |
| iPad mini 6 | 744×1133 | 2 | `splash-1488x2266.png` |
| iPad 9th/10th gen | 810×1080 | 2 | `splash-1620x2160.png` |

The PNGs are committed to `frontend/public/splash/` (total ~428 KB). They are not included in the SW precache (`scripts/stamp-sw-version.mjs` does not hash them) and are not SW-cached at runtime — they are served as ordinary static assets by the CDN.

### One-shot regeneration procedure

The generation script `frontend/scripts/gen-ios-splash.mjs` uses Node.js built-ins and `rsvg-convert` (librsvg, Homebrew). It is **not wired into prebuild or any CI step** — the PNGs are committed and the script is only re-run when device coverage needs updating.

```bash
# Prerequisite: rsvg-convert at /opt/homebrew/bin/rsvg-convert
brew install librsvg       # if not already installed

# From the repo root:
node frontend/scripts/gen-ios-splash.mjs
```

The script generates each PNG by building a wrapper SVG (coral full-bleed rect + flamingo logo paths extracted from `frontend/src/app/icon.svg`, scaled to ~40% of the shorter dimension, centered) and converting it with `rsvg-convert -w <W> -h <H> tmp.svg -o public/splash/splash-<W>x<H>.png`. On partial failure, `process.exitCode` is set to `1` so the caller can detect which devices failed; successfully-generated files are not rolled back.

After regeneration, visually verify each PNG (open in a browser: coral fills to the edges, logo is centered) before committing.
