# `revalidate: 0` for any RSC fetch that depends on the current user

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

Any GraphQL query whose result depends on `Authorization` (role lookups, `me`, owned-resource queries, profile data) must pass `{ revalidate: 0 }` to `gqlFetch`. Caching auth-sensitive data either across users (via Next's data cache key, which does not include the access token) or across role changes (admin role revoked while the cached page is alive) is a correctness bug.

The pattern applies to every RSC, layout, or route handler that calls `gqlFetch` with an auth-dependent query. To find all current call sites: `grep -rn "gqlFetch" frontend/src/`. Any new file added to those results that touches auth-sensitive data must pass `{ revalidate: 0 }` — there is no per-file list to maintain because the enumeration rots the moment a new auth-dependent page lands.

The three `revalidate` states are documented in `docs/frontend.md` — `0` means no cache, `false` means cache forever, omitted means Next's default heuristic. Pick `0` for auth-sensitive; never collapse to a `number` default.
