# Apollo `client.query()` is not cancelled on unmount — guard `setState` with `isMountedRef`

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

Apollo's imperative `client.query()` (and `client.mutate()`) returns a `Promise` that is not tied to the calling component's lifecycle. If the component unmounts while the promise is pending — due to navigation, React Strict Mode's double-mount, or a user closing a tab — the `.then()` callback still fires and may call `setState` on a component that is no longer mounted. In React 18+, calling `setState` on an unmounted component no longer throws, but it does consume CPU and may apply stale data to a new instance of the same component that remounted with different state.

The fix is a `isMountedRef` that tracks the component's live status. The cleanup function of the effect sets it to `false`, and every async callback that calls `setState` gates on it:

```ts
const isMountedRef = useRef(true);
useEffect(() => {
  isMountedRef.current = true;
  return () => {
    isMountedRef.current = false;
  };
}, []); // empty deps — track the component lifetime, not a specific value

useEffect(() => {
  if (someCondition) return;
  client
    .query({ query: SomeDocument, variables, fetchPolicy: "network-only" })
    .then((result) => {
      if (!isMountedRef.current) return;  // guard before any setState
      setState((current) => mergeResult(current, result.data));
    });
}, [someCondition, client]);
```

**Why `useRef`, not `useState`:** a `useState` flag would schedule a re-render on the unmounted component — the exact symptom we are trying to prevent. A `useRef` write is synchronous and produces no re-render, so it is safe to call from a cleanup function.

**Initialise to `true` in the body, restore in cleanup:** `useRef(true)` plus an effect that sets it to `true` on mount and `false` on cleanup is the correct Strict Mode pattern. In Strict Mode, React mounts → unmounts → remounts every component in development. Without the `isMountedRef.current = true` inside the effect, the second mount leaves `isMountedRef.current` as `false` from the first mount's cleanup, causing the guard to reject results from the second mount's requests.

## Separate the concurrency guard from the lifecycle guard

A background prefetch that triggers from a queue-length threshold needs two refs, not one:

- **`prefetchInFlightRef`** (`useRef(false)`) — prevents a second concurrent request while the first is pending. Reset in `.finally()` so a failed attempt does not block future prefetch attempts. This is the concurrency guard.
- **`isMountedRef`** — prevents `setState` from being called after unmount. Reset by the effect cleanup. This is the lifecycle guard.

The two have different reset triggers and must remain separate. Collapsing them into one boolean would make a failed prefetch clear the unmount guard, or make unmounting leave the in-flight guard permanently `true`:

```ts
// Backend prefetch with both guards
const isMountedRef = useRef(true);
useEffect(() => {
  isMountedRef.current = true;
  return () => { isMountedRef.current = false; };
}, []);

const prefetchInFlightRef = useRef(false);
useEffect(() => {
  if (queue.length === 0 || queue.length > PREFETCH_THRESHOLD) return;
  if (prefetchInFlightRef.current) return;            // concurrency guard
  prefetchInFlightRef.current = true;
  client
    .query({ query: LearnNextDueCardsDocument, variables, fetchPolicy: "network-only" })
    .then((result) => {
      if (!isMountedRef.current) return;              // lifecycle guard
      setQueue((current) => mergeIncoming(current, result.data?.learnNextDueCards ?? []));
    })
    .finally(() => {
      prefetchInFlightRef.current = false;            // reset concurrency guard
    });
}, [queue.length, cardgroupId, client]);
```

## Merge-based prefetch vs server-authoritative replace

The prefetch's `.then()` merges incoming cards into the existing queue by id (`seen = new Set(current.map(c => c.id))`), appending only cards whose id is not already present. This deduplicates cards that the server returns on multiple consecutive fetches (e.g. due cards that were not swiped yet appear in both the initial SSR batch and the background refetch).

The `handleSwipe` mutation's `nextCards` response, by contrast, is an authoritative full replace — the server FSRS scheduler owns the queue state after each swipe, so the client writes `nextCards` directly into the queue without merging. The two update strategies coexist in the same component: prefetch merges, swipe replaces. The natural re-fire of the prefetch effect on the next `queue.length` change re-evaluates whether a top-up is still needed.

Reference: `frontend/src/app/learn/[cardgroupId]/learn-client.tsx` (both the `setLastViewedCardgroup` persist effect and the background prefetch effect).
