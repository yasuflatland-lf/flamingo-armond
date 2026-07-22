# `console.error("[scope] getUser() failed:", err.name, err.message)` before throwing in RSC

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

In production, the Next.js App Router strips `error.message` and renders a generic "Application error" page unless the error is a special like `NEXT_REDIRECT`. Operators triaging a failure see only the boundary log, not the underlying cause. Any RSC that rethrows a real auth failure must log first — the shape below is the prescriptive one for a direct `getUser()` caller, which is why the chapter is titled after it:

```ts
if (error && error.name !== "AuthSessionMissingError") {
  console.error("[scope] getUser() failed:", error.name);
  throw error;
}
```

The `[scope]` prefix is the route or component name (`[supabase/middleware]`, `[admin-layout]`, `[healthz]`) so the log is greppable.

**No production code calls `getUser()` today**, so the snippet above has no live call site — it is the contract any future direct caller must honour. Identity is resolved once, in the middleware (`frontend/src/lib/supabase/middleware.ts`, prefix `[supabase/middleware]`), which uses `supabase.auth.getClaims()` as its single auth source and logs `console.error("[supabase/middleware] getClaims() failed:", claimsError.name)` on the error branch. Protected pages — including `frontend/src/app/admin/users/page.tsx` and the standard signed-in surfaces — gate on the middleware-forwarded `x-auth-status` header through the shared gate `requireAuthenticated(target)` (`frontend/src/lib/supabase/auth-status.ts`), which wraps `readAuthContext(await headers())`; `frontend/src/app/login/page.tsx` and `frontend/src/app/layout.tsx` call `readAuthContext` directly because neither may redirect on a non-`authenticated` status. None of them has an auth-error log path.

`frontend/src/app/admin/layout.tsx` gates in two steps. Step 1 is the session check, `await requireAuthenticated("/")`. Step 2 is the role check via `gqlFetch(AdminLayoutMeQuery, { revalidate: 0 })`; its catch delegates the auth-code redirect to the shared `redirectIfAuthError(err, "/", { forbidden: true })` (`@/lib/apollo/graphql-errors`) rather than matching the code inline, and for any other failure logs with prefix `[admin-layout]` and rethrows: `console.error("[admin-layout] gqlFetch failed:", { name: err instanceof Error ? err.name : "unknown" })`. A `me` response whose roles omit `admin` falls through to a plain `redirect("/")` with no log — a non-admin reaching an admin URL is expected traffic, not a failure. The same prefix convention applies to non-auth logs such as `frontend/src/app/api/healthz/route.ts` (prefix `[healthz]`).
