# State ownership for high-frequency callbacks: push state down to the consumer

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A pointer-move callback driven by `useDrag` from `@use-gesture/react`, a `scroll` listener, a `requestAnimationFrame` ticker, or any handler that fires at display-refresh rates (60–120 Hz) propagates one frame's worth of cost wherever it calls `setState`. If the `setState` lives in an ancestor component, the ancestor and every component beneath it down to (but excluding) memoized subtrees reconciles each frame. When the same subtree also contains an animation driven by `@react-spring/web` — itself running on its own rAF loop — the two loops compete for the main thread and the visible result is frame drops, dropped pointer events, and a "sticky" feel to the gesture.

The structural fix is not callback identity stabilization (covered separately) and not memoization of the consumer — it is **moving the state declaration into the component that consumes it**. Reconciliation cost stops at the boundary where the state lives.

```tsx
// Anti-pattern: per-frame state owned by the route-level client component.
// Every onSwipeProgress call from useDrag (one per pointer-move event, ~60 Hz)
// triggers a setState in LearnClient, re-rendering its entire subtree —
// including the SwipeCardStack, the action bar, the floating add-card button,
// and any error-banner JSX — alongside the @react-spring/web rAF loop animating
// the card position. The two loops fight on the main thread and the gesture
// feels sticky on mobile.
export function LearnClient(/* ... */) {
  const [swipeDirection, setSwipeDirection] = useState<SwipeDirection | null>(null);
  const [swipeProgress, setSwipeProgress] = useState(0);
  // ...
  return (
    <SwipeCardStack
      onSwipeProgress={(direction, progress) => {
        setSwipeDirection(direction);
        setSwipeProgress(progress);
      }}
      swipeDirection={swipeDirection}
      swipeProgress={swipeProgress}
      /* ... */
    />
  );
}
```

```tsx
// Correct: per-frame state owned inside SwipeCardStack — the only consumer.
// LearnClient never re-renders for a pointer-move event; reconciliation is
// scoped to the stack itself, leaving the @react-spring/web loop alone on the
// main thread.
export function SwipeCardStack<TCard extends SwipeCardData>(/* ... */) {
  const [swipeDirection, setSwipeDirection] = useState<SwipeDirection | null>(null);
  const [swipeProgress, setSwipeProgress] = useState(0);

  const handleSwipeProgress = useCallback(
    (direction: SwipeDirection | null, progress: number) => {
      setSwipeDirection(direction);
      setSwipeProgress(progress);
    },
    [],
  );
  // ...
  return (
    <div>
      <SwipeCard onSwipeProgress={handleSwipeProgress} /* ... */ />
      <SwipeProgressOverlay direction={swipeDirection} progress={swipeProgress} />
    </div>
  );
}
```

**Why:** React's reconciliation walk starts at the component whose state changed and visits every descendant that is not separated by a `React.memo` boundary or whose props' shallow comparison hits. When the state lives in a route-level client component (`LearnClient`), the walk visits every UI piece on the page on every pointer-move event. Pushing the state into the leaf consumer (`SwipeCardStack`) shrinks the walk to a single component. The visible symptom — sticky drag, dropped frames on mid-range Android — is a direct consequence of how long each reconciliation pass takes, and the route-level walk takes long enough that the rAF callback driving `@react-spring/web` misses its deadline.

Callback identity stabilization (covered in [§ "Stabilize callback identity via `useRef` mirrors when the callback reads frequently-changing state"](./stabilize-callback-identity-via-useref-mirrors.md)) does **not** solve this. Identity stabilization prevents memoized children from re-subscribing to a new function reference — useful for `useEffect` deps, IntersectionObserver subscriptions, and `React.memo` cache hits. It does nothing about the cost of the ancestor's own re-render, because the `setState` call is the trigger and identity stabilization changes only what happens to the function the callback closure produced. The two rules are orthogonal:

| Axis | Identity stabilization (the other rule) | State ownership (this rule) |
|---|---|---|
| What is stabilized | The function reference passed down as a prop | The location of the `useState` declaration |
| Failure mode addressed | Memoized children re-subscribing on every parent render | Ancestor's whole subtree reconciling on every callback invocation |
| Cost paid by | Subscriber's re-subscription / re-mount work | Every component between the state owner and the leaf consumer |
| Right tool when | A callback flows into a `useEffect` dep array or `React.memo` | A high-frequency callback's `setState` is the bottleneck |

The two interact: a 60 Hz callback whose state lives in the consumer **and** whose identity is stable is the only combination that compounds correctly. Lifting the state up while keeping the identity stable still pays the ancestor's reconciliation cost on every frame. Stabilizing the identity while keeping the state up the tree still pays the ancestor's reconciliation cost on every frame. Both rules need to be satisfied.

**How to apply:** when a callback fires at display-refresh rates and writes to React state, identify the **lowest common ancestor of every component that reads the state**. That ancestor — not a higher-level coordinator — is where the `useState` declaration belongs. The ancestor's other props (callbacks that the parent provides for the leaf to call when the gesture commits, not when it progresses) stay at the higher tier. Move only the per-frame state down.

A useful diagnostic: open React DevTools' profiler, record one pointer-move gesture, and inspect which component's "Render reason" lists the per-frame state as a prop or state change. Anything above the leaf consumer is over-rendering and is a candidate for the state-ownership push-down.

The split applies cleanly when the per-frame state has no consumer outside the leaf component. If a sibling needs to read it (e.g. an error banner that lights up when the gesture would commit), prefer publishing a derived snapshot via a stable subscription channel (a `useSyncExternalStore`, a small Zustand store) over hoisting the state back up. The hoist re-introduces the original bottleneck; the external store is the escape hatch for the rare cross-cutting consumer.

Reference: `SwipeCardStack` in `frontend/src/components/learn/swipe-card-stack.tsx` owns `swipeDirection` and `swipeProgress`; `LearnClient` in `frontend/src/app/learn/[cardgroupId]/learn-client.tsx` no longer carries them, and no longer re-renders during a drag gesture.
