# Transient overlay triggered by a mutation whose source row was optimistically removed

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

## Why

A post-action reward overlay (e.g. the "Memory +N%" badge after a swipe) reads two
values: the OLD state from the row that was acted on, and the NEW state from the
mutation payload. When the action also optimistically removes that row from a
client-managed queue *before* the mutation resolves, the overlay can no longer be
a child of the row — the row is already gone. It floats over the stack instead,
reading the OLD value from the swiped card's closure and the NEW value from the
resolved payload.

That floating lifecycle introduces three timing hazards, all amplified by a
background prefetch that can repopulate the same queue at an arbitrary moment.
The hazards live in
[`frontend/src/app/learn/[cardgroupId]/learn-client.tsx`](../../../frontend/src/app/learn/%5BcardgroupId%5D/learn-client.tsx)
and
[`frontend/src/components/learn/memory-grew-overlay.tsx`](../../../frontend/src/components/learn/memory-grew-overlay.tsx).

## What

### 1. Auto-dismiss timer must not reset on an unrelated parent re-render

The overlay's auto-dismiss `useEffect` depends on its `onDone` identity (`[delta, onDone]`
in `MemoryGrewOverlay`). An inline `onDone={() => setReward(null)}` is a fresh
reference on every parent render, so any mid-window re-render — a background
prefetch resolving and calling the queue setter — re-runs the effect, clears the
old `setTimeout`, and restarts the 1500 ms window. Under repeated re-renders the
overlay can fail to dismiss.

Fix: pass a stable handler. The empty-dependency `useCallback` is the simplest
form of the ref-mirror technique:

```tsx
const handleRewardDone = useCallback(() => setReward(null), []);
```

This is the empty-dep degenerate case of
[Stabilize callback identity via `useRef` mirrors](stabilize-callback-identity-via-useref-mirrors.md):
there is no frequently-changing state to read, so no ref is needed — only the
stable identity matters for the consumer's effect dependency.

### 2. A session-ending action must set no reward — guard from inside the queue updater

Setting the reward runs AFTER `await handleSwipe`. On the final card the
optimistic `setQueue` empties the queue first; the component early-returns
`<AllCaughtUp />`, which unmounts the overlay so its `onDone` never fires. Then
the mutation resolves and would set the reward while the queue is already empty.

A clear-on-empty `useEffect` keyed on the `queue.length === 0` transition does
NOT catch this: no transition occurs, because the length was already `0` when the
reward was set. A prefetch that was already in flight (fired while the queue was
still non-empty, and not cancellable) then resolves, repopulates the queue, and
the overlay re-mounts — flashing the previous action's reward over an unrelated
incoming card.

Fix: capture whether the optimistic removal emptied the session from INSIDE the
`setQueue` updater, then skip `setReward` when it did:

```tsx
let sessionEnded = false;
setQueue((current) => {
  const next = current.filter((candidate) => candidate.id !== card.id);
  sessionEnded = next.length === 0;
  return next;
});
// ...after await handleSwipe, in the success branch:
if (!sessionEnded) {
  setReward({ fromStability: ..., toStability: ... });
}
```

Reading the predicate from inside the updater observes the true current queue
(robust to a prefetch resolving between this action and the previous one), and is
idempotent under React StrictMode's double-invoke of the updater.

### 3. Keep the clear-on-empty effect as a backstop

The source guard in (2) covers a reward whose own action ended the session. It
does NOT cover a reward set by an EARLIER non-final action that is still pending
when a LATER action empties the queue. Retain a clear-on-empty `useEffect` keyed
on `queue.length === 0` as the backstop for that case:

```tsx
useEffect(() => {
  if (queue.length === 0) setReward(null);
}, [queue.length]);
```

## Cross-references

- [Stabilize callback identity via `useRef` mirrors](stabilize-callback-identity-via-useref-mirrors.md) —
  the general form of the stable-handler fix in hazard (1).
- [`docs/backend/ddd-patterns/mutation-response-must-not-carry-client-managed-collection.md`](../../backend/ddd-patterns/mutation-response-must-not-carry-client-managed-collection.md) —
  the server side of the same optimistic-queue design: the write response carries
  only the affected entity's post-write state, and the client owns the queue with a
  separate prefetch lifecycle. That separation is exactly what makes the overlay's
  before/after delta computable without a refetch, and what makes the prefetch an
  uncancellable repopulation source the hazards above must tolerate.
