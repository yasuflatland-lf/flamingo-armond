# Auth (Supabase)

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

The Supabase SSR client uses a 3-layer setup mirroring the official `@supabase/ssr` template. Each layer exists because cookie reading/writing differs between contexts:

| Layer | File | Cookie source | Used by |
|---|---|---|---|
| Browser | `src/lib/supabase/client.ts` | `document.cookie` (handled by `createBrowserClient`) | Client components, `authLink` |
| Server (RSC / route handler) | `src/lib/supabase/server.ts` | `next/headers` `cookies()` | Server components, route handlers, `auth/callback/route.ts` |
| Middleware | `src/lib/supabase/middleware.ts` | `NextRequest.cookies` / `NextResponse.cookies` | `src/middleware.ts` (cookie rotation) |

### authLink for browser GraphQL

`src/lib/apollo/client.ts` composes `from([authLink, httpLink])`. `authLink` calls `supabase.auth.getSession()` on every request and attaches `Authorization: Bearer <jwt>` when a session exists. `getSession()` reads from local cookies — it is not an HTTP call, so per-request invocation is cheap. When the JWT is stale, the SDK refreshes internally.

If no session exists the header is omitted (not set to an empty string). Backend treats missing `Authorization` as anonymous.

Backend JWT verification is enabled. Without a Supabase session, only unauthenticated queries (e.g., `health`) succeed against `/query` until resolvers begin enforcing authentication.

### Middleware cookie rotation

`src/middleware.ts` calls `updateSession(request)` from `lib/supabase/middleware.ts`. The implementation calls `supabase.auth.getClaims()` once. `getClaims()` verifies the JWT locally (this project signs with asymmetric keys and auth-js caches the JWKS in a module-global) and, via its internal `getSession()`, refreshes an expiring token — the `setAll` cookie hook captures the rotation. This replaces the former per-navigation `getUser()` call, which always sent an Auth-server round trip purely to re-verify the same token. The `matcher` excludes `_next/static`, `_next/image`, `favicon.ico`, common image extensions, and the static PWA endpoints `sw.js`, `offline.html`, and `manifest.webmanifest`. The PWA exclusions exist because those endpoints carry no session to rotate, so running `getUser()` / cookie-rotation on them is wasted work — `sw.js` especially, since it is re-fetched on every page load due to its `no-cache` header. See [`pwa.md`](pwa.md) for the full PWA context.

The `matcher` must also explicitly exclude `/api/:path*` and `/auth/callback`. Without the `/api` exclusion, every Apollo browser POST to `/api/graphql` triggers a full Supabase token-refresh round-trip in middleware, adding latency per GraphQL call. Without the `/auth/callback` exclusion, middleware cookie writes race against the route handler's own `exchangeCodeForSession` and can corrupt the new session.

#### Middleware-forwarded identity headers

`frontend/src/lib/supabase/middleware.ts` (`updateSession`) is the single source of auth identity for server-side rendering. After calling `getClaims()` (whose internal `getSession()` rotates the session cookie), it derives three server-internal request headers and sets them before passing the request on:

| Header | Type | Source |
|---|---|---|
| `x-auth-status` | `authenticated\|anonymous\|stale\|error` | `getClaims()` result (claims present → `authenticated`; `{ data: null, error: null }` → `anonymous`; error → `stale`/`error` via `isStaleSessionError`) |
| `x-user-email` | string (empty when anonymous) | `claims.email` from `getClaims()` |
| `x-user-is-admin` | `"true"\|"false"` | `getClaims()` → `claims.app_metadata.role === "admin"` |

Any client-supplied copies of these headers are stripped at the top of `updateSession` before the middleware sets its own — a client must never be able to spoof them. The headers are set on the forwarded **request** (not on the response), so the browser never sees them; this is the same posture as `x-nonce` (see [`csp.md`](csp.md)). The root layout reads all three via `readAuthContext` (`frontend/src/lib/supabase/auth-status.ts`) and passes the derived `{ status, email, isAdmin }` props to `AppShell` (via `ConditionalShell`).

#### No page calls `getUser()` directly — every page reads the forwarded header

No production code calls `supabase.auth.getUser()` directly. The middleware (`frontend/src/lib/supabase/middleware.ts`) verifies the JWT via `getClaims()` (above) and forwards `x-auth-status`; every protected page gates on that header through `requireAuthenticated(target)` (`frontend/src/lib/supabase/auth-status.ts`), which wraps `readAuthContext(await headers())` and redirects when the status is not `authenticated` — `/login` for the ordinary surfaces, `/` for the admin surfaces (`app/admin/layout.tsx`, `app/admin/users/page.tsx`). The two sites that must not redirect on a non-`authenticated` status call `readAuthContext` directly: `app/layout.tsx` (derives the shell props) and `app/login/page.tsx` (inverts the check). For the full RSC error-handling contract — including the `AuthSessionMissingError` filter that any future direct `getUser()` caller must apply — see [`.claude/rules/frontend-rsc-error-handling.md`](../../.claude/rules/frontend-rsc-error-handling.md).

### `isAdmin` is read from the JWT claim via the middleware, not from a GraphQL query

The **middleware** (`frontend/src/lib/supabase/middleware.ts`) computes the `isAdmin` flag by calling `supabase.auth.getClaims()` (wrapped in try/catch, fails closed to `false`) and checking `claims.app_metadata?.role === "admin"`. It forwards the result as the `x-user-is-admin` request header. The root layout (`frontend/src/app/layout.tsx`) reads this header via `readAuthContext` (`frontend/src/lib/supabase/auth-status.ts`) and passes `isAdmin` as a prop to `AppShell` (via `ConditionalShell`). The claim is emitted at JWT mint time by the Supabase Custom Access Token Hook, which joins `public.user_roles` server-side — see [`docs/backend/custom-access-token-hook.md`](../backend/custom-access-token-hook.md). The frontend therefore reads the role from the cookie-side JWT (no network round-trip) and skips a GraphQL `me` query on every RSC navigation.

The role lives **only** in the JWT claim — it is NOT mirrored into `auth.users.app_metadata` (the Custom Access Token Hook injects it at mint time without writing to `raw_app_meta_data`), so `getUser().user.app_metadata?.role` always returns `undefined`. Read it via `getClaims()`.

This is a deliberate split: the **middleware + layout** reads the JWT claim as a UI hint to decide nav visibility, and `frontend/src/app/admin/layout.tsx` enforces the gate with a `me`-query role check. A stale or missing JWT claim hides the admin rail but never grants access. See the failure-mode contract in [`.claude/rules/frontend-rsc-error-handling.md`](../../.claude/rules/frontend-rsc-error-handling.md) for the degraded-shell rule, and [`docs/frontend/rsc-error-handling/getclaims-three-way-return.md`](rsc-error-handling/getclaims-three-way-return.md) for the three-way return shape of `getClaims()`.

### Role-change propagation copy: bound by token refresh, not a hardcoded interval

Role changes take effect when the user's JWT is re-minted — at sign-in or at the next token refresh. The refresh cadence is the `jwt_expiry` value in `supabase/config.toml` and the Supabase dashboard, which is operator-tunable (default `3600`). UI copy that informs a user (or an admin editing a user) about the propagation delay MUST NOT hardcode the time bound:

```tsx
// Correct — durable phrasing.
<p>Role changes take effect at the user's next sign-in or token refresh.</p>

// Incorrect — couples copy to a config value.
<p>Role changes take effect within one hour.</p>
```

Reference: `frontend/src/app/admin/users/admin-user-profile-sheet.tsx` (the role-multiselect helper text).

### Gotchas

- **`src/middleware.ts`, not `frontend/middleware.ts`**: Next.js with `src/` layout expects middleware under `src/`.
- **Supabase cookie names**: `sb-<project_ref>-auth-token` (split across `sb-...-auth-token.0` / `.1` for large JWTs). Useful when DevTools-debugging a missing session.
- **`@supabase/ssr` mocking in Vitest**: Real `createBrowserClient` crashes in node env. Use `vi.mock("@supabase/ssr", ...)` to stub `createBrowserClient` / `createServerClient` and return controllable `auth` objects.
- **OAuth redirect: 127.0.0.1 vs localhost**: Google treats them as separate origins. Match what `supabase start` prints (`127.0.0.1`).
- **`createSupabaseServerClient` `setAll` catch is scoped to Server Components.** The helper is also used by Route Handlers (e.g. `auth/callback/route.ts`) where cookie writes DO succeed. The `try/catch` in `setAll` silently swallows errors in both paths; a failure inside a Route Handler would be invisible. Do not repurpose `createSupabaseServerClient` in contexts where a write failure must surface (e.g. a middleware-like flow) without removing or re-throwing from that catch.
- **Return-to open-redirect policy lives in exactly one module.** `/auth/callback` accepts no return-to parameter at all — it redirects unconditionally to `/` (see [`docs/frontend/routing-topology.md` § "HomePage redirect chain"](routing-topology.md#homepage-redirect-chain)). Every other flow that honours a caller-supplied path routes it through `sanitizeReturnTo` (`frontend/src/lib/sanitize-return-to.ts`), the tree's sole open-redirect guard; never re-derive an inline check. `WHATWG URL` accepts absolute URLs and protocol-relative paths even when given a base-URL argument, so a bare `startsWith("/")` test is insufficient. `sanitizeReturnTo` rejects a value unless it starts with `/` and its second character is neither `/` nor `\` — the backslash case matters because http(s) special-scheme parsers normalize `\` → `/`, turning `/\evil.com` into a protocol-relative URL. Note the check is positional (character 1 only), not a whole-string `includes("\\")`: `/foo\bar` is accepted.
