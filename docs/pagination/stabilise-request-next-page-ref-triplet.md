# Stabilise `requestNextPage` with `useEffectEvent` and a same-tick mutex

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

This is the compatibility page for the old ref-triplet wording. The recommended React 19.2 shape for IntersectionObserver pagination is `useEffectEvent`.

The production pattern keeps the request logic in a normal helper and moves the observer-only latest-value read into an Effect Event:

```tsx
const fetchNextPage = useCallback(
  ({ hasNextPage, endCursor, searchQuery }: {
    hasNextPage: boolean;
    endCursor: string | null;
    searchQuery: string | null;
  }) => {
    if (fetchingRef.current || !hasNextPage) return;
    fetchingRef.current = true;
    fetchMore({
      variables: {
        ...DEFAULT_VARS,
        after: endCursor,
        search: searchQuery,
      },
    })
      .then(() => setFetchMoreError(null))
      .catch((err) => { /* ...structured warn + banner... */ })
      .finally(() => {
        fetchingRef.current = false;
      });
  },
  [fetchMore],
);

const requestNextPageFromObserver = useEffectEvent(() => {
  fetchNextPage({ hasNextPage, endCursor, searchQuery });
});

useEffect(() => {
  if (!hasNextPage || fetchMoreError != null) return;
  const node = sentinelRef.current;
  if (!node) return;
  const observer = new IntersectionObserver((entries) => {
    if (!entries[0]?.isIntersecting) return;
    requestNextPageFromObserver();
  });
  observer.observe(node);
  return () => observer.disconnect();
}, [hasNextPage, fetchMoreError]);
```

`useEffectEvent` reads the latest committed `hasNextPage`, `endCursor`, and `searchQuery`, but it is not a stable callback identity. That is fine here because the observer callback is created inside the `useEffect` that owns the subscription. The Effect Event must only be called from Effects or callbacks registered by Effects; do not call it from UI event handlers like Retry buttons.

Keep `fetchingRef` as the explicit same-tick in-flight mutex. `useEffectEvent` does not serialize overlapping `fetchMore` calls, and `useTransition` would not guard synchronous repeated observer fires before pending state commits. `fetchingRef` is retained because it addresses the same-tick serialization problem (preventing duplicate in-flight `fetchMore` calls within a single render tick), which `useEffectEvent` does not solve. The trio (`endCursorRef`, `hasNextPageRef`, `searchQueryRef`) addressed only the stale-closure problem.

Retry should call the parameterized helper directly, not the Effect Event:

```tsx
const retryFetchMore = useCallback(() => {
  fetchNextPage({ hasNextPage, endCursor, searchQuery });
}, [fetchNextPage, hasNextPage, endCursor, searchQuery]);
```

The observer effect keeps structural deps such as `hasNextPage` / `pageInfo.hasNextPage` and `fetchMoreError`. It should not depend on the Effect Event's latest-value reads.

**How to apply:** any paginated hook or client component that subscribes to an IntersectionObserver and advances via `fetchMore` should use this pattern. Today's call sites are `frontend/src/app/cardgroups/cardgroups-client.tsx`, `frontend/src/app/cardgroups/[id]/cards/use-cards-connection.ts` (consumed by `cards-client.tsx`), and `frontend/src/app/admin/users/admin-users-client.tsx`. New paginated hooks should keep the helper parameterized and keep the observer path Effect-owned; do not reintroduce the cursor/search/hasNextPage ref triplet unless you are in a non-Effect callback that cannot call an Effect Event.
