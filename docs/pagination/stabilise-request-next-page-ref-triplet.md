# Stabilise `requestNextPage` via the cursor / search / hasNextPage ref triplet

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

The `useCallback` for `requestNextPage` reads three values that change every time a page lands or the user types into the search box: `endCursor`, `searchQuery`, and `hasNextPage`. If any of them appear in the callback's dep array, the callback gets a fresh identity on every advance — and the IntersectionObserver `useEffect` (which lists `requestNextPage` as a dep) tears down and re-attaches the observer on every page transition. The fix is to mirror all three values into refs and read them via `*.current` inside the callback, leaving only Apollo's stable `fetchMore` (and any cardgroup-id parameter) in the dep array:

```tsx
const endCursorRef = useRef(endCursor);
const searchQueryRef = useRef(searchQuery);
const hasNextPageRef = useRef(hasNextPage);
useEffect(() => { endCursorRef.current = endCursor; }, [endCursor]);
useEffect(() => { searchQueryRef.current = searchQuery; }, [searchQuery]);
useEffect(() => { hasNextPageRef.current = hasNextPage; }, [hasNextPage]);

const requestNextPage = useCallback(() => {
  if (fetchingRef.current || !hasNextPageRef.current) return;
  fetchingRef.current = true;
  fetchMore({
    variables: {
      ...DEFAULT_VARS,
      after: endCursorRef.current,
      search: searchQueryRef.current,
    },
    /* ... */
  })
    .then(() => setFetchMoreError(null))
    .catch((err) => { /* ...structured warn + banner... */ })
    .finally(() => { fetchingRef.current = false; });
}, [fetchMore]); // stable identity across page advances + search-text edits

useEffect(() => {
  if (!hasNextPage || fetchMoreError != null) return;
  const node = sentinelRef.current;
  if (!node) return;
  const observer = new IntersectionObserver((entries) => {
    if (!entries[0]?.isIntersecting || fetchingRef.current) return;
    requestNextPage();
  });
  observer.observe(node);
  return () => observer.disconnect();
}, [hasNextPage, fetchMoreError, requestNextPage]);
```

**Why apply all three refs together, not just one:** mirroring only `endCursor` while leaving `searchQuery` in the dep array still re-creates the callback on every keystroke; mirroring only `searchQuery` still re-creates it on every page advance. The triplet is the minimal stable set: `endCursor` advances per page, `searchQuery` changes per debounced input, `hasNextPage` flips when the last page is reached. The observer effect's structural deps (`hasNextPage`, `fetchMoreError`) are intentionally still real deps — those are the conditions that should re-subscribe the observer. Dropping `hasNextPage` from the effect's dep array would leave the observer attached past the last page; the rule keeps `hasNextPage` as a structural dep AND mirrors it into a ref so the callback's runtime check reads the current value without taking a per-advance dep.

**How to apply:** any paginated hook or client component that subscribes to an IntersectionObserver and advances via `fetchMore` MUST apply the triplet. Today's call sites are `frontend/src/app/cardgroups/cardgroups-client.tsx`, `frontend/src/app/cardgroups/[id]/cards/use-cards-connection.ts` (consumed by `cards-client.tsx`), and `frontend/src/app/admin/users/AdminUsersClient.tsx`. New paginated hooks should follow the same shape from the start — keep the `requestNextPage` dep array narrow to `[fetchMore]` (plus any caller-stable scalar like `cardgroupId`) and mirror every state value the body reads. This rule extends [the IntersectionObserver in-flight guard](intersection-observer-in-flight-guard.md): that rule covers the boolean re-entrancy guard; this rule covers the cursor/search/hasNextPage cohort that drives the callback's identity.
