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

`src/middleware.ts` calls `updateSession(request)` from `lib/supabase/middleware.ts`. The implementation MUST call `supabase.auth.getUser()` once — without it Supabase does not refresh expiring tokens and the session silently drops. The `matcher` excludes `_next/static`, `_next/image`, `favicon.ico`, common image extensions, and the static PWA endpoints `sw.js`, `offline.html`, and `manifest.webmanifest`. The PWA exclusions exist because those endpoints carry no session to rotate, so running `getUser()` / cookie-rotation on them is wasted work — `sw.js` especially, since it is re-fetched on every page load due to its `no-cache` header. See [`pwa.md`](pwa.md) for the full PWA context.

The `matcher` must also explicitly exclude `/api/:path*` and `/auth/callback`. Without the `/api` exclusion, every Apollo browser POST to `/api/graphql` triggers a full Supabase token-refresh round-trip in middleware, adding latency per GraphQL call. Without the `/auth/callback` exclusion, middleware cookie writes race against the route handler's own `exchangeCodeForSession` and can corrupt the new session.

### `isAdmin` is read from the JWT claim, not from a GraphQL query

The root layout (`frontend/src/app/layout.tsx`) computes the `isAdmin` flag forwarded to `AppShell` from `claims.app_metadata?.role === "admin"` returned by `supabase.auth.getClaims()`. The claim is emitted at JWT mint time by the Supabase Custom Access Token Hook, which joins `public.user_roles` server-side — see [`docs/backend/custom-access-token-hook.md`](../backend/custom-access-token-hook.md). The frontend therefore reads the role from the cookie-side JWT (no network round-trip) and skips a GraphQL `me` query on every RSC navigation.

This is a deliberate split: the **layout** reads the JWT claim as a UI hint to decide nav visibility, and `frontend/src/app/admin/layout.tsx` enforces the gate with a `me`-query role check. A stale or missing JWT claim hides the admin rail but never grants access. See the failure-mode contract in [`.claude/rules/frontend-rsc-error-handling.md`](../../.claude/rules/frontend-rsc-error-handling.md) for the degraded-shell rule, and [`docs/frontend/rsc-error-handling/getclaims-three-way-return.md`](rsc-error-handling/getclaims-three-way-return.md) for the three-way return shape of `getClaims()`.

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
- **Open redirect via `?next=`.** `WHATWG URL` accepts absolute URLs and protocol-relative paths even when given a base-URL argument; a bare `startsWith("/")` check is insufficient. The correct guard: `value.startsWith("/") && !value.startsWith("//") && !value.includes("\\")`. The backslash variant bypasses naive checks because http(s) special-scheme parsers normalize `\` → `/`.
