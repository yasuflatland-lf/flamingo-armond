# Suspense fallback does not catch thrown errors — wrap async server components in try/catch

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

`<Suspense>` only catches *suspensions* — promises that a component throws to signal it is not yet ready. It does **not** catch thrown `Error` objects. An async server component inside a `<Suspense>` boundary that `throw`s an error (rather than awaiting a pending promise) escapes the boundary entirely and propagates to the nearest React error boundary (`error.tsx` or `global-error.tsx`). If neither exists, Next.js serves a 500.

This distinction matters most for transport-level failures. The Supabase SDK returns most errors via the resolved `{ data, error }` shape — those never throw and are handled by normal conditional logic. But a **transport-level rejection** (DNS lookup failure, connection refused, JWKS endpoint hang, TLS error) causes the SDK `await` to reject. That rejection becomes a thrown error inside the component, escaping any `<Suspense>` above it.

## The pattern: degrade inside the component, never let transport errors propagate

Async server components that perform network I/O inside a `<Suspense>` boundary must wrap their awaits in `try/catch` and degrade gracefully on transport rejections rather than re-throwing. Re-throwing escapes the boundary and, for root-level components with no parent `error.tsx`, 500s the entire site.

A representative example is an async RSC data-loader component that runs inside a `<Suspense>` boundary and degrades to empty/default state on transport failure:

```tsx
// Hypothetical async RSC wrapped in <Suspense fallback={<Skeleton />}>
export async function SomeDataContent() {
  let data: SomeData | null = null;

  try {
    data = await fetchSomeData();
  } catch (err) {
    // A transport-level rejection (DNS failure, connection refused) makes
    // the fetch reject rather than return an {error}.
    // Suspense does not catch thrown errors and there may be no error.tsx
    // ancestor — degrade to the empty state instead of 500-ing.
    console.error("[some-data] fetch threw; degrading to empty state:", {
      name: err instanceof Error ? err.name : "unknown",
    });
  }

  return <SomeDataView data={data} />;
}
```

Note: `frontend/src/components/auth-shell.tsx` previously held this pattern but is now a **synchronous prop-driven wrapper** (`{ user, isAdmin, children }`) that performs no auth I/O. The equivalent degradation logic moved to `frontend/src/lib/supabase/middleware.ts`, which wraps the `getClaims()` call in a try/catch and fails closed to `isAdmin=false` so the app stays up on transport failures. See [`getclaims-three-way-return.md`](getclaims-three-way-return.md).

## Relation to the "auth gate runs outside Suspense" rule

The existing rule in [`auth-outside-suspense-boundary.md`](auth-outside-suspense-boundary.md) says: when a route's auth check must `redirect()` on failure, run that check in the outer `page.tsx` RSC before the `<Suspense>` element is returned. The motivation is that a redirect inside a suspended subtree causes the skeleton fallback to flash before the navigation fires.

These two rules address **different components with different responsibilities**:

| Component | Role | Allowed to redirect? | Inside Suspense? |
|---|---|---|---|
| Route `page.tsx` outer RSC | Auth *gate* — must enforce access | Yes — redirects on failure | No — runs before the `<Suspense>` return |
| Root layout + `AuthShell` | Display-hint *shell* — reads middleware-forwarded headers via `readAuthContext`; passes `user` / `isAdmin` props | No — degrades to anonymous | No — `AuthShell` is synchronous (no auth I/O); no Suspense boundary needed |

The auth gate (page outer RSC) runs outside Suspense because it needs to redirect, and redirects from inside a suspended subtree flash the fallback first. `AuthShell` is now a synchronous prop-driven wrapper — identity resolution moved to the middleware (out of the React render path entirely), so the root layout reads pre-computed headers via `readAuthContext` and the Suspense boundary was removed.

Both rules share the same underlying constraint: `<Suspense>` does not catch thrown errors. The auth gate addresses this by staying outside Suspense entirely. Async RSC data-loaders that live inside Suspense boundaries address it by wrapping their awaits in `try/catch`. Which approach applies depends on whether the component's failure mode is "redirect" (stay outside) or "degrade" (stay inside, catch).

## Why `app/loading.tsx` does not solve the streaming problem

`loading.tsx` places the `page` in a Suspense boundary so the page skeleton streams while the page RSC resolves. But `loading.tsx` cannot help when the **layout** itself blocks before returning any JSX. A layout that `await`s network I/O before its `return` statement holds the entire HTML flush — no `<Suspense>` fallback, no `loading.tsx` skeleton, nothing streams until the layout's `return` executes. See [`docs/frontend/pwa.md` § "PWA white-screen fix: de-blocking the root layout (Interval B)"](../pwa.md#pwa-white-screen-fix-de-blocking-the-root-layout-interval-b) for the full analysis and the fix.
