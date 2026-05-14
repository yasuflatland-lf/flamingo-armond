# `revalidate: 0` for any RSC fetch that depends on the current user

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

Any GraphQL query whose result depends on `Authorization` (role lookups, `me`, owned-resource queries, profile data) must pass `{ revalidate: 0 }` to `gqlFetch`. Caching auth-sensitive data either across users (via Next's data cache key, which does not include the access token) or across role changes (admin role revoked while the cached page is alive) is a correctness bug. Used today in `frontend/src/app/layout.tsx` (via `AppShell`), `frontend/src/app/page.tsx`, `frontend/src/app/cards/new/page.tsx`, `frontend/src/app/admin/layout.tsx`, `frontend/src/app/profile/page.tsx`, and `frontend/src/app/api/healthz/route.ts` (the last one for probe freshness, not auth, but the constant is the same).

The three `revalidate` states are documented in `docs/frontend.md` — `0` means no cache, `false` means cache forever, omitted means Next's default heuristic. Pick `0` for auth-sensitive; never collapse to a `number` default.
