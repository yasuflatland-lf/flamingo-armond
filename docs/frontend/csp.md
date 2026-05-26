# CSP and security headers

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

## Scope

The frontend ships an **enforcing** CSP for HTML responses, a **static offline-page CSP**, and a small reporting endpoint that accepts browser violation payloads and logs them server-side. The policy builders live in [`frontend/src/lib/security/csp.ts`](../../frontend/src/lib/security/csp.ts), the middleware hook is in [`frontend/src/lib/supabase/middleware.ts`](../../frontend/src/lib/supabase/middleware.ts), the static header split is defined in [`frontend/next.config.ts`](../../frontend/next.config.ts), and the report sink is [`frontend/src/app/api/csp-report/route.ts`](../../frontend/src/app/api/csp-report/route.ts).

## Policy builders

[`frontend/src/lib/security/csp.ts`](../../frontend/src/lib/security/csp.ts) exports two public pieces:

1. `serializeCsp(directives)` takes a directive map and turns it into a stable header string. It drops `null` / `undefined` / `false`, trims sources, removes duplicates, and emits directives in the configured order with any unknown directives sorted alphabetically after the known ones.
2. `buildHtmlCsp({ nonce, supabaseUrl, reportUri?, reportTo?, speedInsightsOrigin? })` builds the policy used for HTML routes. It requires a non-empty nonce, derives the Supabase HTTPS origin plus the matching realtime WebSocket origin from `supabaseUrl`, and includes the default reporting endpoints.

The HTML policy currently allows:

- `default-src 'self'`
- `base-uri 'self'`
- `object-src 'none'`
- `frame-ancestors 'none'`
- `form-action 'self'`
- `img-src 'self' data: blob: https://lh3.googleusercontent.com`
- `style-src 'self' 'unsafe-inline'`
- `script-src 'self' 'nonce-<generated>'`
- `connect-src 'self' <supabase https origin> <supabase wss origin> https://vitals.vercel-insights.com`
- `report-uri /api/csp-report`
- `report-to csp-endpoint`

That is the current shipped policy shape. `style-src 'unsafe-inline'` remains because the existing animation and styling approach still depends on inline CSS. The Google avatar origin is intentionally whitelisted in `img-src`, and the Vercel Speed Insights origin is intentionally whitelisted in `connect-src`.

The shipped `/offline.html` CSP in [`frontend/next.config.ts`](../../frontend/next.config.ts) is:

- `default-src 'none'`
- `script-src 'none'`
- `style-src 'unsafe-inline'`
- `img-src 'self'`
- `base-uri 'none'`
- `form-action 'none'`

## Middleware flow

[`frontend/src/lib/supabase/middleware.ts`](../../frontend/src/lib/supabase/middleware.ts) generates a fresh nonce per request, builds the HTML policy, and does two different things with it:

1. It forwards the policy into the request headers as `Content-Security-Policy` and exposes the nonce as `x-nonce`. That forwarded header is for Next's server-side rendering path, where the nonce is extracted for script tags before the browser ever sees the response.
2. It sets the browser-visible response header to `Content-Security-Policy`, so the policy is enforced and violations are reported to the configured endpoint.

The middleware also forwards `x-pathname` for server components and preserves the same forwarded CSP / nonce headers when the Supabase client refreshes cookies.

If `buildHtmlCsp` throws (e.g., malformed `NEXT_PUBLIC_SUPABASE_URL`), the middleware logs via `console.error` and omits both CSP headers rather than crashing all requests. Auth, cookie rotation, and routing continue to work; only CSP enforcement and violation reporting is absent.

The policy is now **enforcing** on HTML responses. Violations are blocked and reported to the configured endpoint.

## Static headers

[`frontend/next.config.ts`](../../frontend/next.config.ts) splits headers by path:

1. HTML routes matching `"/((?!api/|_next/|sw\\.js$|offline\\.html$|.*\\.).*)"` receive the shared security header set:
   - conservative HSTS: `Strict-Transport-Security: max-age=31536000`
   - `Referrer-Policy: strict-origin-when-cross-origin`
   - `Permissions-Policy: camera=(), geolocation=(), microphone=(), payment=(), usb=()`
   - `X-Content-Type-Options: nosniff`
   - `X-Frame-Options: DENY`
   - `Reporting-Endpoints: csp-endpoint="/api/csp-report"`
2. `/offline.html` receives the same shared header set plus a hardcoded inline CSP string (this is separate from any builder function).
3. `/sw.js` receives `Content-Type`, a no-cache `Cache-Control`, and `X-Content-Type-Options`.

The HSTS choice is deliberately conservative: it ships without preload and without subdomain coverage. `Reporting-Endpoints` is already shipped so user agents that prefer the Reporting API can map the `csp-endpoint` token to `/api/csp-report`.

## Report endpoint contract

[`frontend/src/app/api/csp-report/route.ts`](../../frontend/src/app/api/csp-report/route.ts) reads the request body as text, lowercases the top-level content type for logging, parses JSON-shaped payloads, normalizes them, logs valid reports with `console.error` (security signal), and always returns `204`.

Current tests cover `application/csp-report`, `application/reports+json`, and generic `application/json` payloads. The route does not branch on content type; it accepts either a single object or an array of objects, and it also accepts the legacy nested `{"csp-report": ...}` shape. Both dash-case and camelCase field names are normalized into a compact log envelope with:

- `type`
- optional `age`
- optional `url`
- optional `userAgent`
- a normalized `body` object with the CSP fields the browser supplied

Malformed JSON, empty bodies, and unexpected shapes are still answered with `204`; they are just logged as invalid payloads.

## Current decisions

These are the decisions currently shipped in code:

- CSP is enforcing for HTML routes.
- `style-src 'unsafe-inline'` is retained.
- Supabase uses both its HTTPS origin and its WSS realtime origin in `connect-src`.
- Vercel Speed Insights is allowed in `connect-src`.
- Google avatar images from `https://lh3.googleusercontent.com` are allowed in `img-src`.
- `Reporting-Endpoints` is enabled in the static header set.
- HSTS is present but conservative.

Future enforcement or report aggregation work should stay brief in this doc until it is actually implemented. Today the system only logs violation reports; nothing aggregates them or triggers alerts.

## Verification

Ground the doc against the implementation with:

```bash
rtk pnpm --filter frontend test src/lib/security/csp.test.ts src/lib/supabase/middleware.test.ts src/app/api/csp-report/route.test.ts
```

Manual checks to run in a local environment that can actually reach the dev server:

1. Start `pnpm --filter frontend dev`.
2. Open a page that renders HTML and confirm the response carries `Content-Security-Policy` plus the forwarded `Content-Security-Policy` and `x-nonce` middleware headers.
3. Confirm `/offline.html` serves the static CSP and `/sw.js` serves the no-cache worker headers.
4. Trigger a CSP violation and confirm the browser POSTs to `/api/csp-report` and the server logs a normalized report.

In environments where localhost TCP access is restricted, run these checks on a machine that can reach the dev server.
