# Split debounce from immediate-reset effects on the same input

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

A single effect that combines the debounced state update with the immediate-reset cleanup violates the "one effect, one synchronization" guideline AND silently delays the reset by the debounce window. The user starts typing, the prior page's `fetchingRef = true` and `fetchMoreError` banner remain in place for 300ms, and the IO observer continues to interpret the prior cursor as live during that window. Split into two effects keyed on different triggers — the input for the debounce, the resulting query for the reset:

```tsx
// frontend/src/app/admin/users/AdminUsersClient.tsx
// Debounce: update searchQuery 300ms after the last keystroke.
useEffect(() => {
  const timer = setTimeout(() => {
    setSearchQuery(searchInput.trim() || null);
  }, 300);
  return () => clearTimeout(timer);
}, [searchInput]);

// When the active search query changes, drop any in-flight guard and stale error
// banner immediately so the new query starts from a clean slate.
// biome-ignore lint/correctness/useExhaustiveDependencies: searchQuery is an intentional trigger dependency; it is not referenced in the body because the effect resets derived IO state, not searchQuery itself.
useEffect(() => {
  fetchingRef.current = false;
  setFetchMoreError(null);
}, [searchQuery]);
```

**Why:** the two effects synchronize different things. The debounce effect synchronizes `searchQuery` to `searchInput` with a delay. The reset effect synchronizes `fetchingRef` / `fetchMoreError` to `searchQuery` with no delay. Combining them into one timer ties the reset's timing to the debounce's timing for no reason, which means a stale fetchMore banner stays visible during the entire debounce window even though the user has clearly moved on. Splitting them lets each synchronization run on its own trigger.

**How to apply:** every paginated client component that has both (a) a debounced search-input → search-query pipeline and (b) IO observer reset state (`fetchingRef`, `fetchMoreError`) MUST use two separate effects. Today's call sites are `frontend/src/app/cardgroups/cardgroups-client.tsx`, `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx`, and `frontend/src/app/admin/users/AdminUsersClient.tsx` — all three follow the split-effect shape. The `biome-ignore lint/correctness/useExhaustiveDependencies` comment on the reset effect is load-bearing (see [load-bearing-biome-ignore.md](load-bearing-biome-ignore.md)) — do not remove it during cleanup.
