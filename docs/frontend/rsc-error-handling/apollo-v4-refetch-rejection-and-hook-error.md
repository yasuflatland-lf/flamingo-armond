# A failed `void refetch()` surfaces via the `useQuery` hook's `error` state — no local transport-error machinery needed

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

A fire-and-forget `void refetch()` from a `useQuery` hook is a complete retry
story on its own: a transport failure surfaces through the hook's `error` field,
and the discarded promise rejection is pre-handled by Apollo. A component does
**not** need its own try/catch around `refetch()`, a local `useState` error flag,
or any other transport-error state machinery to recover. Render an error-first
branch off the hook's `error` and a failed refetch lands there.

The contract has two independent channels:

1. **Observable emission → hook `error`.** A refetch transport failure is
   delivered to the `ObservableQuery`'s subscribers as a normal emission, not as
   a thrown exception. In `@apollo/client@4.2.0`, `ObservableQuery.js`
   (~lines 1281–1297) turns a network error notification into an emission with
   `error` set, `networkStatus: NetworkStatus.error`, and `loading: false`. The
   `useQuery` hook subscribes to that observable, so its `error` field becomes
   populated on the next render.

2. **Promise rejection → pre-handled.** The promise returned by `refetch()` is
   wrapped by Apollo's `preventUnhandledRejection` (a no-op catch attached
   internally). Discarding it with `void refetch()` therefore produces **no**
   unhandled-rejection warning — the rejection is already consumed before the
   call site ignores it.

**Conditions.** This holds with the default `errorPolicy` ("none") and
`notifyOnNetworkStatusChange: true` so the hook re-renders on the
`loading → error` networkStatus transition. (Without `notifyOnNetworkStatusChange`
the `error` still lands, but an intermediate `loading` flip may not re-render;
the practice client sets the flag.)

**Practical consequence.** `void refetch()` plus an error-first render branch
(`if (error) return <Banner onRetry={() => { void refetch(); }} />`) is a full
retry loop. There is no need to await the refetch, catch its rejection, or mirror
the failure into a local state field — the hook already owns the error state, and
the next render shows the banner. `frontend/src/app/learn/[cardgroupId]/practice-client.tsx`
records this contract in comments at both `void refetch()` sites (the `studyAgain`
restart and the `retry` handler) and renders the error banner ahead of the
skeleton and completion screens.

**Verification method.** This contract was settled against a primary source, not
inferred from docs: an empirical vitest probe that drove a failing refetch and
asserted the hook's `error` populated, cross-checked against the installed
`@apollo/client@4.2.0` source (`ObservableQuery.js` and the
`preventUnhandledRejection` wrapper). The line numbers and behavior are
version-pinned to `@apollo/client@4.2.0`; re-verify on any major-version upgrade,
as the emission path and the rejection-suppression wrapper are internal and may
move.

Related: [`isUnauthenticatedGraphQLError` matches gqlFetch — not Apollo Client runtime errors](apollo-runtime-vs-gqlfetch-error-shape.md)
covers reading `extensions.code` off a client-runtime error via `liftGraphQLCodes`;
the practice client uses that helper to log the failed refetch's codes inside the
`error`-keyed effect. For the imperative `client.query()` counterpart (which is
**not** lifecycle-bound and needs an `isMountedRef` guard), see
[`Apollo client.query() is not cancelled on unmount`](apollo-client-query-unmount-guard.md).
