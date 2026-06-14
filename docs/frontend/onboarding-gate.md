# Onboarding gate

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

The "is this user onboarded?" question is asked twice — once by HomePage (to decide whether to send the user *into* `/onboarding`) and once by `/onboarding` itself (to decide whether to send an already-onboarded caller back *out*). Both questions resolve to the same predicate: `isUserOnboarded(me)` in `frontend/src/lib/auth/onboarding.ts`. The predicate is the unit of meaning; the two pages are two consumers.

## Why HomePage owns the gate

HomePage is the canonical post-sign-in landing — the OAuth callback, an already-signed-in `/login` hit, and the bare URL `/` all converge there. Branching on onboarding state at `/auth/callback` instead would create a second decision point that has to repeat the `lastViewedCardgroup` lookup, the `myCardgroupsConnection.totalCount` check, and the predicate itself. Two decision points means two places that can drift, and the second place has no shell affordance to surface the drift to a developer — a returning user would silently land on the wrong screen depending on which entry they used.

Putting the gate at HomePage and defaulting `/auth/callback`'s post-exchange redirect to `/` (not `/cardgroups`) makes the redirect chain a single ordered walk. See [`routing-topology.md` § "HomePage redirect chain"](./routing-topology.md#homepage-redirect-chain) for the full chain.

## Why the predicate is shared between HomePage and OnboardingPage

A naïve implementation would only gate at HomePage and leave `/onboarding` reachable by direct URL — a user who has already filled in `displayName` could deep-link `/onboarding` and sit on a no-op form, or worse, overwrite their existing profile through a second submit.

The fix is structural, not procedural: `/onboarding` self-guards by calling the *same* `isUserOnboarded(me)` predicate and redirecting onboarded callers to `/`. Sharing the predicate (rather than copy-pasting `me?.displayName?.trim().length > 0` into both pages) makes "what counts as onboarded?" a single fact. Future tightenings — requiring an avatar, requiring email verification, etc. — flow to both gates by editing one helper. A copy-pasted check would silently let one gate accept a stricter definition than the other and re-open the deep-link bypass.

```
/onboarding (self-guard)
├─ no Supabase session          → /login
├─ isUserOnboarded(me)          → /              (HomePage runs the chain again)
└─ else                         → render OnboardingForm
```

The form (`frontend/src/app/onboarding/onboarding-form.tsx`) follows the same TanStack Form + Apollo `useMutation` + `getBackendFieldErrors` / `getBackendErrorBanner` template as `profile-form.tsx`. See [`profile-page-profile.md` § "Form library"](./profile-page-profile.md#form-library) for the template; only the differences are documented here.

## Why `/onboarding` bypasses `AppShell`

`/onboarding` is in `ConditionalShell`'s `BARE_ROUTES` set (`frontend/src/components/conditional-shell.tsx`) alongside `/login` and the first-deck chooser `/onboarding/start`. The user has no `displayName` yet — a header that shows their email next to an empty name slot would surface the very state the page is asking them to fix, and the rail's Profile link would offer a competing edit path that has nothing to do with the onboarding flow. Bare-shell focuses the viewport on the single decision the user needs to make.

Because the page bypasses `AppShell`, it renders its own `<main>` landmark — see [`rsc-error-handling/pages-bypassing-appshell-must-render-main.md`](./rsc-error-handling/pages-bypassing-appshell-must-render-main.md).
