# `console.error("[scope] getUser() failed:", err.name, err.message)` before throwing in RSC

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

In production, the Next.js App Router strips `error.message` and renders a generic "Application error" page unless the error is a special like `NEXT_REDIRECT`. Operators triaging a failure see only the boundary log, not the underlying cause. RSCs that rethrow a real `getUser()` failure must log first:

```ts
if (error && error.name !== "AuthSessionMissingError") {
  console.error("[home] getUser() failed:", error.name, error.message);
  throw error;
}
```

The `[scope]` prefix is the route or component name (`[home]`, `[login]`, `[layout]`, `[cards-new]`, `[cardgroups-new]`, `[healthz]`) so the log is greppable. Used today in `frontend/src/app/page.tsx`, `frontend/src/app/login/page.tsx`, `frontend/src/app/layout.tsx`, `frontend/src/app/cards/new/page.tsx`, `frontend/src/app/cardgroups/new/page.tsx`, and `frontend/src/app/api/healthz/route.ts`.
