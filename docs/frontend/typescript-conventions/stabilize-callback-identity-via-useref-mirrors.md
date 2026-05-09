# Stabilize callback identity via `useRef` mirrors when the callback reads frequently-changing state

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A `useCallback` whose body reads from a stateful value listed in its dep array gets a fresh identity every time that value changes. When the callback flows down to a child that subscribes to it (e.g. a global keydown listener, an IntersectionObserver, a memoized child component), the subscription is torn down and rebuilt on every state change. The fix is to mirror the state into a ref, list the ref-owning effect as the only dep on the value, and have the callback read `ref.current`:

```tsx
// frontend/src/components/learn/swipe-card-stack.tsx — keydown listener
const activeCardRef = useRef(activeCard);
useEffect(() => {
  activeCardRef.current = activeCard;
}, [activeCard]);

// triggerSwipe stays stable across activeCard changes — no dep on activeCard.
const triggerSwipe = useCallback(
  (direction: SwipeDirection) => {
    if (!activeCardRef.current) return;
    onCardSwiped(activeCardRef.current, direction);
  },
  [onCardSwiped],
);

// keydown listener subscription does not re-register on every card.
useEffect(() => {
  function onKeyDown(event: KeyboardEvent) {
    if (!activeCardRef.current) return;
    /* ... */
  }
  window.addEventListener("keydown", onKeyDown);
  return () => window.removeEventListener("keydown", onKeyDown);
}, [triggerSwipe]);
```

The same shape applies to a `useCallback` that reads from the rendered queue / cursor / search-text but should NOT be re-created when the value advances:

```tsx
// frontend/src/app/learn/[cardgroupId]/learn-client.tsx — onSwipe callback
const queueRef = useRef(queue);
useEffect(() => {
  queueRef.current = queue;
}, [queue]);

const onSwipe = useCallback(
  async (card, direction) => {
    const remaining = queueRef.current
      .filter((c) => c.id !== card.id)
      .map(withTypename);
    /* ...build optimisticResponse with `remaining`, fire mutation... */
  },
  [cardgroupId, handleSwipe], // queue removed from deps via queueRef
);
```

**Why:** `useCallback` identity is what React uses to decide whether a child needs to re-subscribe (`useEffect` dep arrays, `React.memo` shallow-equal). A callback that re-creates on every queue update propagates that churn down through every memoized consumer, defeating the memoization. The ref mirror is a one-line indirection that removes the value from the dep array without losing access to it. The `useEffect` that writes the ref is the only place the value is observed, and writes to a ref do not trigger renders or downstream re-subscriptions.

**How to apply:** any `useCallback` that (a) reads from frequently-changing state AND (b) flows to a consumer that re-subscribes on identity change should use a ref mirror. The trigger is "is the callback's identity load-bearing for a consumer's subscription?" — if the answer is yes, mirror. The IntersectionObserver case (cursor / search / hasNextPage triplet) is documented in `.claude/rules/pagination.md` § "Stabilise `requestNextPage` via the cursor / search / hasNextPage ref triplet"; the keydown and queue cases above are non-IO uses of the same shape. Test the stability with a behavioural assertion (e.g. capture the callback in a `vi.fn()` wrapper and assert `toHaveBeenCalledTimes(1)` across multiple state updates) — see `frontend/src/app/learn/[cardgroupId]/learn-client.test.tsx` (`onCardSwiped` reference stability across swipes) and `frontend/src/components/learn/swipe-card-stack.test.tsx` (keydown listener stability across `activeCard` changes).
