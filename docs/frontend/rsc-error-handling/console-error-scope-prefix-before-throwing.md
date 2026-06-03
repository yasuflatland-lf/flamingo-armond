# `console.error("[scope] getUser() failed:", err.name, err.message)` before throwing in RSC

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

In production, the Next.js App Router strips `error.message` and renders a generic "Application error" page unless the error is a special like `NEXT_REDIRECT`. Operators triaging a failure see only the boundary log, not the underlying cause. RSCs that rethrow a real `getUser()` failure must log first:

```ts
if (error && error.name !== "AuthSessionMissingError") {
  console.error("[supabase/middleware] getUser() failed:", error.name);
  throw error;
}
```

The `[scope]` prefix is the route or component name (`[supabase/middleware]`, `[admin-layout]`, `[healthz]`) so the log is greppable. The sole `getUser()` call site is the middleware (`frontend/src/lib/supabase/middleware.ts`, prefix `[supabase/middleware]`). All other protected pages — including `frontend/src/app/login/page.tsx`, `frontend/src/app/admin/users/page.tsx`, and most standard pages — read the middleware-forwarded `x-auth-status` header via `readAuthContext` and have no `getUser()` error path. `frontend/src/app/admin/layout.tsx` performs its second-step role check via `gqlFetch(AdminLayoutMeQuery)`; when that call fails with an auth code it redirects to `/`, and for any other failure it logs with prefix `[admin-layout]` and rethrows: `console.error("[admin-layout] gqlFetch failed:", { name: err instanceof Error ? err.name : "unknown" })`. The same prefix convention applies to non-auth logs such as `frontend/src/app/api/healthz/route.ts` (prefix `[healthz]`).
