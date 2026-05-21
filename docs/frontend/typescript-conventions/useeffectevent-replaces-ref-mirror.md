# `useEffectEvent` replaces ref mirrors for Effect-owned callbacks

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

React 19.2 `useEffectEvent` is the preferred replacement for the old `useRef` mirror pattern when a callback is owned by an Effect or by a callback registered from an Effect.

Use it when you need a subscription callback to read the latest committed values without forcing the subscription effect to re-register on every state change:

```tsx
const requestNextPageFromObserver = useEffectEvent(() => {
  fetchNextPage({ hasNextPage, endCursor, searchQuery });
});

useEffect(() => {
  const observer = new IntersectionObserver((entries) => {
    if (!entries[0]?.isIntersecting) return;
    requestNextPageFromObserver();
  });
  observer.observe(sentinelRef.current!);
  return () => observer.disconnect();
}, [hasNextPage, fetchMoreError]);
```

`useEffectEvent` reads the latest committed state and props, but it is not a stable callback identity. That is acceptable here because the callback is created and consumed inside the Effect-owned subscription path. Do not pass an Effect Event to UI handlers, memoized children, or other places that need stable identity.

For pagination, keep the real request logic in a normal parameterized helper:

```tsx
const fetchNextPage = useCallback(
  ({ hasNextPage, endCursor, searchQuery }: FetchNextPageArgs) => {
    if (fetchingRef.current || !hasNextPage) return;
    fetchingRef.current = true;
    // call fetchMore, clear/record fetchMoreError, finally release fetchingRef
  },
  [fetchMore],
);
```

Use that helper directly from Retry buttons and other UI event handlers. UI handlers must not call the Effect Event.

Keep `fetchingRef` as an explicit same-tick mutex for the in-flight window. `useEffectEvent` solves stale reads, not same-tick serialization. Synchronous repeated observer fires can still enqueue overlapping `fetchMore` calls before React commits pending state, and `useTransition` does not close that gap.

This pattern replaces the old cursor/search/hasNextPage ref triplet for IntersectionObserver pagination. The ref-mirror pattern is still a valid fallback for non-Effect callbacks that cannot be called from an Effect-owned subscription, but it is no longer the recommended shape for pagination.
