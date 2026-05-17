# `revalidate: 0` for any RSC fetch that depends on the current user

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

Any GraphQL query whose result depends on `Authorization` (role lookups, `me`, owned-resource queries, profile data) must pass `{ revalidate: 0 }` to `gqlFetch`. Next's data cache key does not include the access token, so caching auth-sensitive data leaks it across users; an admin role revoked while a cached page is alive is the same bug in time.

To find all current call sites: `grep -rn "gqlFetch" frontend/src/`. Any new file that touches auth-sensitive data must pass `{ revalidate: 0 }` — there is no per-file list to maintain because the enumeration rots the moment a new auth-dependent page lands.

The three `revalidate` states are documented in `frontend/CLAUDE.md` — `0` means no cache, `false` means cache forever, omitted means Next's default heuristic. Pick `0` for auth-sensitive; never collapse to a `number` default.
