# `console.error("[scope] getUser() failed:", err.name, err.message)` before throwing in RSC

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

Next.js may abbreviate or replace thrown errors in production (the App Router strips error.message in prod and renders a generic "Application error" page unless the error is a `NEXT_REDIRECT` or similar special). Operators triaging a failure see only the boundary log, not the underlying cause. RSCs that rethrow a real `getUser()` failure must log first:

```ts
if (error && error.name !== "AuthSessionMissingError") {
  console.error("[home] getUser() failed:", error.name, error.message);
  throw error;
}
```

The `[scope]` prefix is the route or component name (`[home]`, `[login]`, `[global-header]`, `[cards-new]`, `[cardgroups-new]`, `[healthz]`) so the log is greppable. Used today in `frontend/src/app/page.tsx`, `frontend/src/app/login/page.tsx`, `frontend/src/components/nav/global-header.tsx`, `frontend/src/app/cards/new/page.tsx`, `frontend/src/app/cardgroups/new/page.tsx`, and `frontend/src/app/api/healthz/route.ts`.
