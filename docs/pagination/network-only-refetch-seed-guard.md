# Seeding local state from a `network-only` query: gate on `loading` AND result identity

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

## Why

With `fetchPolicy: "network-only"` and `notifyOnNetworkStatusChange: true`, an in-flight `refetch()` does **not** clear `data` — Apollo retains the *previous* result while the new request is on the wire. A `useEffect` that copies query data into local component state ("seed a local round queue from `data.practiceTodaysCards`") will therefore observe the **stale pool** during the refetch window, not the fresh one. If the seed copies indiscriminately, two regressions follow:

- **Mid-refetch stale seed.** While `loading` is `true`, the retained old array flashes the just-finished cards back into the local queue before the new pool arrives.
- **Re-seed clobbers local shrinkage.** The local queue legitimately shrinks as the user swipes cards away. An unrelated re-render that carries the *same* settled `data` reference re-runs the effect and overwrites the shrunk queue with the full original pool.

Both are invisible in the SSR-prop data-loading mode; they appear only because a single query-driven mode refetches in place and Apollo's in-place retention makes "did `data` change?" a subtler question than "is `data` present?".

## The two-condition seed guard

The seed effect must gate on **both** a freshness condition and an identity condition:

```tsx
const pool = data?.practiceTodaysCards;
const seededForRef = useRef<readonly PracticeCard[] | null>(null);

useEffect(() => {
  if (loading) return;                       // (1) never seed from retained stale data mid-refetch
  if (!pool) return;
  if (seededForRef.current === pool) return; // (2) never re-seed the same settled pool array
  seededForRef.current = pool;
  setQueue([...pool]);
}, [pool, loading]);
```

1. **`if (loading) return`** — only settled data seeds. During an in-flight refetch `pool` still points at the old array; skipping while `loading` prevents the stale flash.
2. **`seededForRef.current === pool`** (object identity of the result array) — each settled pool seeds exactly once. A same-reference re-render is a no-op, so the locally-shrunk queue survives until a genuinely new pool array arrives. A restart (`studyAgain` / `retry`) resets `seededForRef.current = null` so even a structurally-identical pool re-seeds the next settled round.

## Companion render rule: skeleton over stale terminal screen

The same retention bites the render path. When a restart is in flight (`loading` true) and the local queue has already been cleared to `[]`, the retained stale `data` is still non-empty — so a naive fall-through reaches a data-derived terminal branch (the completion / "all caught up" screen) and the restart button looks like a no-op, inviting a double-click. Place a skeleton guard on `loading && queue.length === 0` **before** the data-derived branches and **after** the error branch:

```tsx
if (error) return <ErrorBanner onRetry={retry} />;
if (loading && queue.length === 0) return <LearnSkeleton />; // before any data-derived branch
// ...only now branch on (pool?.length ?? 0) === 0 and queue.length === 0
```

Error first (a failed initial load or failed restart shows Retry, never a dead end), then the loading-with-empty-queue skeleton, then the data-derived terminal states.

## Why this is distinct from "Pick one data-loading mode per component"

The [Pick one data-loading mode per component](../../.claude/rules/pagination.md#frontend-cache-patterns) bullet warns against *mixing* two loading modes in one component — an SSR-prop variant (`{ initial: T }`) and a query-id variant (`{ id; query }`) — where `useState(props.initial ?? "")` initializes once and never reflects the resolving query. That is a **mode-selection** bug: the component should be single-mode.

This rule applies **within** the single query-driven mode that bullet recommends. The component here is correctly single-mode (one `useQuery`, no SSR prop); the hazard is **refetch-time staleness** — Apollo retaining the prior `data` across an in-flight refetch — which the mode-selection rule does not address. Choosing one mode is necessary but not sufficient: a single-mode query-driven component that seeds local state still needs the `loading` + identity guard above.

Reference: `frontend/src/app/learn/[cardgroupId]/practice-client.tsx` — the seed effect (`if (loading) return; ... if (seededForRef.current === pool) return;`), the `loading && queue.length === 0` skeleton guard placed after the error branch and before the data-derived terminal branches, and the `studyAgain` / `retry` handlers that reset `seededForRef.current = null` before discarding the `refetch()` promise.
