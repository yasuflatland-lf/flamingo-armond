# IntersectionObserver in-flight guard via `useRef<boolean>`

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

The in-flight guard must live in `useRef<boolean>`, not `useState`. State updates are async — the observer can fire twice in the same animation frame and both passes read the previous `false`, double-firing `fetchMore`. A mutable ref is set synchronously, reset in `.finally`, and never schedules a re-render.

**Reset the guard ref AND the error banner when the active filter changes.** A debounced search input that drives the query's `search` variable is a second axis of "the previous in-flight cursor is now stale": between the user starting to type and the debounced `searchQuery` update, a `fetchMore` call carrying the prior page's `endCursor` may resolve into a different result-set's edge list, or fail mid-flight and leave the IO loop halted on a `fetchMoreError` banner that is no longer relevant to what the user is now searching for. The fix is a `useEffect` keyed on the active filter that clears both ref and banner state:

```ts
// when the active search query changes, drop any in-flight guard + stale error.
useEffect(() => {
  fetchingRef.current = false;
  setFetchMoreError(null);
}, [searchQuery]);
```

The dep array intentionally lists only the trigger (`searchQuery`); the body does not read it. Add a `biome-ignore lint/correctness/useExhaustiveDependencies` comment naming the trigger-not-read intent so the rule does not silently re-engage with future code-mod tools. Reference: `frontend/src/app/cardgroups/cardgroups-client.tsx`.
