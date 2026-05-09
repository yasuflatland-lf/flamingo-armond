# Auth (Supabase)

> Part of [`docs/frontend.md`](../frontend.md). See the index for related chapters.

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

`src/middleware.ts` calls `updateSession(request)` from `lib/supabase/middleware.ts`. The implementation MUST call `supabase.auth.getUser()` once — without it Supabase does not refresh expiring tokens and the session silently drops. The `matcher` excludes `_next/static`, `_next/image`, `favicon.ico`, and common image extensions.

The `matcher` must also explicitly exclude `/api/:path*` and `/auth/callback`. Without the `/api` exclusion, every Apollo browser POST to `/api/graphql` triggers a full Supabase token-refresh round-trip in middleware, adding latency per GraphQL call. Without the `/auth/callback` exclusion, middleware cookie writes race against the route handler's own `exchangeCodeForSession` and can corrupt the new session.

### Gotchas

- **`src/middleware.ts`, not `frontend/middleware.ts`**: Next.js with `src/` layout expects middleware under `src/`.
- **Supabase cookie names**: `sb-<project_ref>-auth-token` (split across `sb-...-auth-token.0` / `.1` for large JWTs). Useful when DevTools-debugging a missing session.
- **`@supabase/ssr` mocking in Vitest**: Real `createBrowserClient` crashes in node env. Use `vi.mock("@supabase/ssr", ...)` to stub `createBrowserClient` / `createServerClient` and return controllable `auth` objects.
- **OAuth redirect: 127.0.0.1 vs localhost**: Google treats them as separate origins. Match what `supabase start` prints (`127.0.0.1`).
- **`createSupabaseServerClient` `setAll` catch is scoped to Server Components.** The helper is also used by Route Handlers (e.g. `auth/callback/route.ts`) where cookie writes DO succeed. The `try/catch` in `setAll` silently swallows errors in both paths; a failure inside a Route Handler would be invisible. Do not repurpose `createSupabaseServerClient` in contexts where a write failure must surface (e.g. a middleware-like flow) without removing or re-throwing from that catch.
- **Open redirect via `?next=`.** `WHATWG URL` accepts absolute URLs and protocol-relative paths even when given a base-URL argument; a bare `startsWith("/")` check is insufficient. The correct guard: `value.startsWith("/") && !value.startsWith("//") && !value.includes("\\")`. The backslash variant bypasses naive checks because http(s) special-scheme parsers normalize `\` → `/`.

