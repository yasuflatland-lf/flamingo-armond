# Mid-session UNAUTHENTICATED in a client component: degraded banner with `<Link href="/login">`, not `redirect()`

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

A client component (`"use client"`) that observes `UNAUTHENTICATED` from an Apollo `useQuery` cannot call `redirect()` from `next/navigation` — that helper is RSC-only and throws a runtime error inside a client tree. Calling `router.replace("/login")` is also wrong: the user may have unsaved local state (search input, half-typed form, scroll position) that vanishes on a hard navigation, and the redirect fires from a `useEffect` that produces the same brief flash described in [§ "Initial-load UNAUTHENTICATED redirects belong in the RSC `page.tsx`"](./initial-load-unauthenticated-redirects-in-rsc.md).

The right shape is a **degraded banner** that surfaces the session-expired message inline and points at `/login` via a `<Link>` the user clicks when they are ready:

```tsx
// frontend/src/app/admin/users/AdminUsersClient.tsx
const queryErrorKind = classifyQueryError(queryError);

// UNAUTHENTICATED post-mount means the session expired while the page was
// open. The server-side gate in page.tsx + the admin layout already block
// the initial load (which redirects to "/"), so this only fires mid-session.
// Render a degraded banner pointing to /login rather than calling
// `redirect()` from a client component — see this rule.
{queryErrorKind?.kind === "unauthenticated" && (
  <div role="alert">
    <span>Your session has expired. </span>
    <Link href="/login" className="underline">Please sign in again.</Link>
  </div>
)}
```

The `classifyQueryError` helper (in `frontend/src/lib/apollo/errors.ts`) returns a typed `QueryErrorKind` discriminated union — `forbidden`, `unauthenticated`, or `banner` — so the JSX can branch on `kind` without re-implementing the structural extension parse at every call site.

**Why:** the initial-load case is already gated by the RSC `page.tsx` + the admin layout ([§ above](./initial-load-unauthenticated-redirects-in-rsc.md)), so a mid-session UNAUTHENTICATED only fires when the access token expires while the page is alive. The user is already signed in to Supabase locally; a forced redirect throws away their in-page state and takes them to a login form they may not have asked for. The degraded-banner posture surfaces the session expiry, names the recovery, and lets the user choose when to navigate. This is the dual of the RSC rule: server-side initial load is "redirect immediately"; client-side mid-session is "render the banner."

**How to apply:** any client component that issues an auth-required query via Apollo MUST classify `queryError` via `classifyQueryError` and render a `<Link href="/login">` banner for the `unauthenticated` branch. Do not import `redirect` from `next/navigation` into a client component. Do not call `router.replace("/login")` from an effect keyed on the error. Reference: `frontend/src/app/admin/users/AdminUsersClient.tsx` (`queryErrorKind?.kind === "unauthenticated"` banner). The same posture applies to a `FORBIDDEN` mid-session case — render a permission-denied banner without a Retry button (re-issuing the query would fail again).
