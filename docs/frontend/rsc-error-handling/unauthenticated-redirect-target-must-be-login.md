# GraphQL UNAUTHENTICATED must redirect to `/login`, not to an auth-required route

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

When a GraphQL query returns `UNAUTHENTICATED`, the RSC must redirect to `/login` — not to another route that itself requires authentication. Redirecting to an auth-required route (e.g. `/cardgroups`) creates a two-hop chain: the first redirect lands the user on the intermediate route, which then runs its own auth check and redirects again to `/login`. The user experiences two navigations instead of one, and server logs record two 307s for a single failed request.

The failure is subtle because it looks correct during development: the final destination is always `/login`. The problem surfaces in staging metrics (double redirect latency), in log correlation (the two-hop leaves the request-ID chain split across two route entries), and occasionally in browser-tab history (the user sees the intermediate URL flicker in the address bar before landing on `/login`).

```tsx
// WRONG — /cardgroups also requires auth, so this produces a 2-hop chain.
if (isUnauthenticatedGraphQLError(err)) redirect("/cardgroups");

// CORRECT — goes directly to the unauthenticated entry point.
if (isUnauthenticatedGraphQLError(err)) redirect("/login");
```

**Why the target must be `/login` specifically:** the routing topology in [`routing-topology.md` § "HomePage redirect chain"](../routing-topology.md#homepage-redirect-chain) establishes `/login` as the canonical unauthenticated entry. After sign-in, `/login` redirects to `/` (HomePage), which then routes the user to the correct post-login destination based on their current state (`lastViewedCardgroup`, cardgroup count, onboarding status). Sending a mid-session UNAUTHENTICATED directly to `/cardgroups` bypasses that decision chain and may land the user in a loop or on the wrong screen.

**How to apply:** every `isUnauthenticatedGraphQLError(err)` catch arm in an RSC page must call `redirect("/login")`. The only exceptions are pages that are themselves auth-free landing pages (e.g. `/login`, `/`) — those never call `gqlFetch` with an auth requirement in the first place. Today's call sites: `frontend/src/app/cardgroups/page.tsx`, `frontend/src/app/cards/new/page.tsx`, `frontend/src/app/learn/[cardgroupId]/page.tsx`. Use `grep -rn "isUnauthenticatedGraphQLError" frontend/src/` to find all current sites and verify each redirects to `/login`.
