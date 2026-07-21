# GraphQL UNAUTHENTICATED must redirect to `/login`, not to an auth-required route

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

When a GraphQL query returns `UNAUTHENTICATED`, the RSC must redirect to `/login` — not to another route that itself requires authentication. Redirecting to an auth-required route (e.g. `/cardgroups`) creates a two-hop chain: the first redirect lands the user on the intermediate route, which then runs its own auth check and redirects again to `/login`. The user experiences two navigations instead of one, and server logs record two 307s for a single failed request.

The failure is subtle because it looks correct during development: the final destination is always `/login`. The problem surfaces in staging metrics (double redirect latency), in log correlation (the two-hop leaves the request-ID chain split across two route entries), and occasionally in browser-tab history (the user sees the intermediate URL flicker in the address bar before landing on `/login`).

```tsx
// WRONG — /cardgroups also requires auth, so this produces a 2-hop chain.
redirectIfAuthError(err, "/cardgroups");

// CORRECT — goes directly to the unauthenticated entry point.
redirectIfAuthError(err, "/login");
```

**Why the target must be `/login` specifically:** the routing topology in [`routing-topology.md` § "HomePage redirect chain"](../routing-topology.md#homepage-redirect-chain) establishes `/login` as the canonical unauthenticated entry. After sign-in, `/login` redirects to `/` (HomePage), which then routes the user to the correct post-login destination based on their current state (`lastViewedCardgroup`, cardgroup count, onboarding status). Sending a mid-session UNAUTHENTICATED directly to `/cardgroups` bypasses that decision chain and may land the user in a loop or on the wrong screen.

**How to apply:** never hand-roll the classify-and-redirect arm. Two named helpers own the decision, and both take the redirect target as an explicit argument so a wrong target is a reviewable diff rather than a copy-paste typo:

- `requireAuthenticated(target)` in `frontend/src/lib/supabase/auth-status.ts` — the RSC auth gate. It reads the middleware-forwarded `x-auth-status` header and redirects when the request is not `authenticated`, returning the `AuthContext` for pages that also need `email` / `isAdmin`.
- `redirectIfAuthError(err, target, { forbidden? })` in `frontend/src/lib/apollo/graphql-errors.ts` — the `catch`-arm classifier. It redirects on `UNAUTHENTICATED`, and additionally on `FORBIDDEN` when `forbidden` is set. It owns only the redirect decision: logging, re-throwing and any degrade-to-a-default fallback stay at the call site, because those differ per page.

The target for an ordinary signed-in surface is `/login`. The only exceptions are pages that are themselves auth-free landing pages (e.g. `/login`, `/`) — those never call `gqlFetch` with an auth requirement in the first place.

The admin surface is the one sanctioned divergence: the `/admin` layout and its pages pass `/` as the target and set `{ forbidden: true }`, collapsing `UNAUTHENTICATED` and `FORBIDDEN` into a single guard — see [`.claude/rules/frontend-rsc-error-handling.md` § "Structurally parse GraphQL `extensions.code` — never substring-match the message"](../../../.claude/rules/frontend-rsc-error-handling.md#structurally-parse-graphql-extensionscode--never-substring-match-the-message). The dominant case there is `FORBIDDEN` (a signed-in non-admin), which has to land somewhere in-app rather than at sign-in; `/` then routes the visitor by state. A genuinely session-less admin visitor does pay a hop through `/`, which is the price of the collapsed guard — do not copy the pattern onto a page whose only auth failure mode is `UNAUTHENTICATED`.

Use `grep -rn "redirectIfAuthError\|requireAuthenticated" frontend/src/app/` to enumerate the call sites and verify each redirect target. Deliberately no list is kept here: the helper signature is the contract, and an enumerated list of pages goes stale the moment a route is added.
