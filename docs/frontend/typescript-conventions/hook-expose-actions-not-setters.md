# Custom hook return shape: expose actions, not raw `useState` setters

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A custom hook usually encapsulates an invariant — "the IntersectionObserver loop halts while an error is set", "the optimistic snapshot is restored when the rollback fires", "the in-flight guard is released after the request settles". Exposing a raw `useState` setter in the hook's result type hands callers a tool that bypasses that invariant. Even with good intent, an external `setX(null)` from a consumer drops the hook back into a state the hook itself would never produce — e.g. error cleared without the matching retry, observer re-enabled without a request in flight.

The fix is to expose **action methods** that perform the state transition the hook intends, and keep the raw setter private. Read-only state (e.g. `fetchMoreError: string | null` for banner display) stays on the result; the mutator does not.

```ts
// Anti-pattern: setFetchMoreError is exposed, letting any caller clear the
// error without re-firing the fetchMore request. The IO loop is re-enabled
// (because the gate checks fetchMoreError != null) but no request is in
// flight, so the observer spins on the sentinel indefinitely.
export interface UseCardsConnectionResult {
  edges: CardEdge[];
  fetchMoreError: string | null;
  setFetchMoreError: (v: string | null) => void; // dangerous
  retryFetchMore: () => void;
  // ...
}

// In a consumer somewhere:
useEffect(() => {
  if (someCondition) result.setFetchMoreError(null); // observer now spins
}, [someCondition]);
```

```ts
// Correct: setFetchMoreError stays private inside the hook. The public surface
// offers retryFetchMore (clears error AND re-fires requestNextPage) plus
// fetchMoreError as a read-only display value. The immediate-reset on
// searchQuery change is internal — callers do not need to know it exists.
export interface UseCardsConnectionResult {
  edges: CardEdge[];
  fetchMoreError: string | null;
  retryFetchMore: () => void;
  // ...
}

// Inside the hook body — setFetchMoreError is local state only.
const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);

const retryFetchMore = useCallback(() => {
  setFetchMoreError(null);
  requestNextPage();
}, [requestNextPage]);
```

**Why:** the hook's invariant is "the IntersectionObserver loop runs only while `fetchMoreError == null` AND a real cause is responsible for it being null — either the immediate-reset on `searchQuery` change, or `retryFetchMore` re-firing the request". Exposing `setFetchMoreError` makes the "AND a real cause" clause unenforceable; any caller can satisfy the first half of the conjunction without the second. The bug never surfaces in the hook's tests because the hook itself only writes to the setter in the documented paths. It surfaces in the consumer the first time a code reviewer suggests "just clear the error from the parent when X happens".

**How to apply:** when designing a hook's result type, name every field after the **consumer-visible outcome** (`fetchMoreError`, `retryFetchMore`, `clearSelection`, `toggleSelected`), never after the underlying React primitive (`setX`, `dispatchY`). Any state transition that requires more than a single `setState` call — even if today it happens to be a single call — belongs behind an action method, because the hook is the only place that can guarantee the surrounding invariants. Reference: `UseCardsConnectionResult` in `frontend/src/app/cardgroups/[id]/cards/use-cards-connection.ts` removed `setFetchMoreError` from the result; `retryFetchMore` is the only public path that clears the error, and the `searchQuery`-keyed effect that also clears it is internal.

This is the frontend analog of the backend convention in [`.claude/rules/error-wrapping.md` § "Legacy primitives (resolver-only)"](../../../.claude/rules/error-wrapping.md#legacy-primitives-resolver-only) — typed error constructors expose only the methods that preserve the wire-format invariants, while the underlying struct literal is forbidden. The principle is the same: a public API that hands callers the raw mutator forfeits every invariant the wrapper exists to maintain.
