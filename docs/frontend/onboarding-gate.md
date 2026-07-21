# Onboarding gate

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

The "is this user onboarded?" question is asked in three places — by the **middleware**, which enforces the invariant on every route; by HomePage, which decides whether to send the user *into* `/onboarding`; and by `/onboarding` itself, which decides whether to send an already-onboarded caller back *out*. All three resolve to the same predicate: `isUserOnboarded(me)` in `frontend/src/lib/auth/onboarding.ts`. The predicate is the unit of meaning; the three sites are three consumers.

## The middleware gate closes the deep-link path

A per-page gate only covers the pages that implement it, and a layout-level gate only covers the routes nested under that layout — so a route added later outside it silently re-opens the bypass. The gate therefore lives in the **middleware** (`frontend/src/lib/supabase/middleware.ts`, delegating to `resolveOnboardingGate` in `frontend/src/lib/auth/onboarding-gate.ts`): one check, ahead of any RSC work, covering every path the matcher in `frontend/src/middleware.ts` reaches, including paths that do not exist yet.

Why this matters is a product fact, not tidiness: a display name is shown to **other** users. A deck published, a catalog entry created, or an admin action taken by an account with an empty `displayName` renders as a blank author to everyone else.

### Exemptions

The gate never redirects these, matched on the exact path or a `/`-bounded sub-path:

| Exempt | Why |
|---|---|
| `/onboarding`, `/onboarding/start` | The redirect target itself. Unexempted, this is an infinite redirect loop. |
| `/login` | Sign-in must stay reachable; it is also where sign-out lands. |
| `/auth` | The OAuth callback has to finish its code exchange. |
| `/api` | Route handlers answer machines — a 307 would break `/api/healthz` and `/api/ping`. |
| `/_next` | Framework assets and RSC payload fetches. |
| `/terms`, `/privacy` | Public legal pages, readable in any account state. |
| `/favicon.ico`, `/sw.js`, `/offline.html`, `/manifest.webmanifest` | Static single-file routes served from `public/`. |

The middleware matcher already excludes `/api`, `/auth/callback`, `/_next` and the static files. The overlap is deliberate — matcher and gate are two independent lists, and the gate must stay correct if the matcher widens.

### Signed fast-path cookie

`displayName` is **not** a JWT claim (the Custom Access Token Hook injects only the admin role), so a naive middleware gate would cost a backend round trip on every matched request. The gate instead reads an integrity-protected `fa-onboarded` cookie and falls back to the backend lookup only when that cookie does not verify:

1. Cookie verifies against this request's `sub` → allow, no I/O.
2. Otherwise fetch `me { id displayName }` from `BACKEND_URL` with the caller's bearer token and evaluate `isUserOnboarded`.
3. Onboarded → allow **and** issue a fresh cookie. Not onboarded → redirect to `/onboarding` and delete the cookie.

The cookie carries `v1.<exp>.<mac>` where `mac = HMAC-SHA256(ONBOARDING_GATE_SECRET, "v1:<sub>:<exp>")` — see `frontend/src/lib/auth/onboarding-cookie.ts`. Three properties follow from that shape:

- **Not forgeable.** The user is the adversary here (the whole point is that they cannot keep an empty display name), so a bare boolean cookie would be a one-line bypass. The MAC is verified with a constant-time compare.
- **Not transferable.** The MAC covers `sub`, so another account's cookie simply fails to verify and the gate falls through to the lookup. An account switch needs no explicit invalidation.
- **Bounded staleness.** The expiry is inside the MAC, so a display name cleared out-of-band (an admin edit) re-gates within the TTL at worst. Verification also rejects an expiry further out than the current TTL, so shortening the TTL takes effect immediately rather than one old window later.

On sign-out or session expiry the middleware deletes the cookie: the branch runs whenever the request is not `authenticated` and the cookie is present, so nothing client-side has to remember to clear an `httpOnly` cookie.

`ONBOARDING_GATE_SECRET` is **optional**. When it is unset the gate still enforces the invariant correctly — it just cannot trust a cookie it cannot authenticate, so every gated navigation pays the lookup and a one-shot warning is logged. Fail-loud-and-correct beats fail-fast-and-broken here: a required secret would take a deployment down until an operator set it. The production bring-up does register it, though: the `make setup-prod` Vercel phase generates the secret with `openssl rand -hex 32`, persists it in the state file, and pushes it to the Vercel project alongside `BACKEND_URL`, so the shipped default is the fast path. See [`env-vars.md`](./env-vars.md) and [`../deployment.md` § "Step 3 — Vercel"](../deployment.md#step-3--vercel).

### Fail open, deliberately

Every unknown outcome — transport failure, non-2xx, GraphQL errors, no access token — allows the request. This gate protects a UX flow; the backend still owns its own authorization, and a backend outage must not strand every user on an `/onboarding` page that cannot load either. A "not onboarded" verdict is only ever reached from a successful lookup that returned a real, empty `displayName`.

The gate's `me` query is a hand-written string rather than a codegen `graphql()` document, because printing a `TypedDocumentNode` would pull graphql-js into the Edge middleware bundle for one three-field query. `onboarding-gate.test.ts` validates the string against `schema/*.graphql`, so it cannot drift from the schema unnoticed.

## Why HomePage owns the gate

HomePage is the canonical post-sign-in landing — the OAuth callback, an already-signed-in `/login` hit, and the bare URL `/` all converge there. Branching on onboarding state at `/auth/callback` instead would create a second decision point that has to repeat the `lastViewedCardgroup` lookup, the `myCardgroupsConnection.totalCount` check, and the predicate itself. Two decision points means two places that can drift, and the second place has no shell affordance to surface the drift to a developer — a returning user would silently land on the wrong screen depending on which entry they used.

Putting the gate at HomePage and defaulting `/auth/callback`'s post-exchange redirect to `/` (not `/cardgroups`) makes the redirect chain a single ordered walk. See [`routing-topology.md` § "HomePage redirect chain"](./routing-topology.md#homepage-redirect-chain) for the full chain.

## Why the predicate is shared across all three gates

A naïve implementation would only gate at HomePage and leave `/onboarding` reachable by direct URL — a user who has already filled in `displayName` could deep-link `/onboarding` and sit on a no-op form, or worse, overwrite their existing profile through a second submit.

The fix is structural, not procedural: `/onboarding` self-guards by calling the *same* `isUserOnboarded(me)` predicate and redirecting onboarded callers to `/`. Sharing the predicate (rather than copy-pasting `me?.displayName?.trim().length > 0` into each site) makes "what counts as onboarded?" a single fact. Future tightenings — requiring an avatar, requiring email verification, etc. — flow to the middleware gate and both pages by editing one helper. A copy-pasted check would silently let one gate accept a stricter definition than the other and re-open the deep-link bypass; the middleware gate is the site where that drift would be least visible, since it never renders anything.

```
/onboarding (self-guard)
├─ no Supabase session          → /login
├─ isUserOnboarded(me)          → /              (HomePage runs the chain again)
└─ else                         → render OnboardingForm
```

The form (`frontend/src/app/onboarding/onboarding-form.tsx`) follows the same TanStack Form + Apollo `useMutation` + `getBackendFieldErrors` / `getBackendErrorBanner` template as `profile-form.tsx`. See [`profile-page-profile.md` § "Form library"](./profile-page-profile.md#form-library) for the template; only the differences are documented here.

## Why `/onboarding` bypasses `AppShell`

`/onboarding` is not in `ConditionalShell`'s content-route allowlist (`SHELL_ROUTE_PREFIXES` in `frontend/src/components/conditional-shell.tsx`), so it renders bare — alongside `/login` and the first-deck chooser `/onboarding/start`. The user has no `displayName` yet — a header that shows their email next to an empty name slot would surface the very state the page is asking them to fix, and the rail's Profile link would offer a competing edit path that has nothing to do with the onboarding flow. Bare-shell focuses the viewport on the single decision the user needs to make.

Because the page bypasses `AppShell`, it renders its own `<main>` landmark — see [`rsc-error-handling/pages-bypassing-appshell-must-render-main.md`](./rsc-error-handling/pages-bypassing-appshell-must-render-main.md).
