# `console.error("[scope] getUser() failed:", err.name, err.message)` before throwing in RSC

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

In production, the Next.js App Router strips `error.message` and renders a generic "Application error" page unless the error is a special like `NEXT_REDIRECT`. Operators triaging a failure see only the boundary log, not the underlying cause. RSCs that rethrow a real `getUser()` failure must log first:

```ts
if (error && error.name !== "AuthSessionMissingError") {
  console.error("[login] getUser() failed:", error.name, error.message);
  throw error;
}
```

The `[scope]` prefix is the route or component name (`[supabase/middleware]`, `[login]`, `[admin]`, `[admin/users]`, `[healthz]`) so the log is greppable. The `getUser()` call sites are: the middleware (`frontend/src/lib/supabase/middleware.ts`, prefix `[supabase/middleware]`), `frontend/src/app/login/page.tsx` (prefix `[login]`), `frontend/src/app/admin/layout.tsx` (prefix `[admin]`), and `frontend/src/app/admin/users/page.tsx` (prefix `[admin/users]`). The same prefix convention applies to non-auth logs such as `frontend/src/app/api/healthz/route.ts` (prefix `[healthz]`). Standard pages do not call `getUser()` and therefore have no `getUser()` error log; they read the middleware-forwarded `x-auth-status` header via `readAuthContext`.
