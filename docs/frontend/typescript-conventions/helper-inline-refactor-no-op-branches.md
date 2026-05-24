# Audit collapsed helpers for branches that lose all side effects

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

When a refactor inlines a multi-purpose helper into a single call site and drops one of the helper's responsibilities at the same time, the surviving guard can leave a branch with no observable effect at all. The original helper combined two side effects under one shared guard:

```ts
// before — helper that ran two updates under one shape-check
function reconcileQueue(data: FetchItemsMutation | null | undefined) {
  if (data?.fetchItems) {
    setQueue(data.fetchItems.items);
    setPerformance(data.fetchItems.performanceMode);
  }
}
```

A "drop the performance-mode UI" refactor inlines the helper and removes `setPerformance`. The naive transcription leaves a `data?.fetchItems`-shaped guard with only `setQueue` inside — and the *else* arm becomes a pure silent no-op:

```ts
// after — the else arm is now a pure silent no-op; no log, no rollback, no UI signal
if (result?.data?.fetchItems) {
  setQueue(result.data.fetchItems.items);
}
```

If `fetchItems` resolves without a payload (mutation completed, server response missing the field), the optimistic queue silently becomes the source of truth and the operator has no way to triage the gap. The fix is an explicit `else if` that distinguishes the rollback path from the missing-data path, with a `console.warn` for operator triage:

```ts
if (result?.data?.fetchItems) {
  setQueue(result.data.fetchItems.items);
} else if (result !== null) {
  // Mutation resolved (no .catch), but the server payload is missing fetchItems.
  // The optimistic queue is now the source of truth; surface for operator triage.
  console.warn("[ItemsClient] fetchItems resolved without data", {
    itemId: item.id,
    collectionId,
  });
}
```

The `result !== null` discriminator distinguishes "mutation rejected and `.catch` returned `null`" (rollback already happened) from "mutation resolved but the payload is incomplete" (the case worth warning about). Without the discriminator, the rejected path triggers the warn redundantly.

**Why:** removing one of two side effects from a shared guard converts the guard from "do thing A and thing B together" to "do thing A or do nothing", and "do nothing" is rarely what the original guard's else case meant. The original `reconcileQueue` had nothing in the else case because both updates were always-together; once they split, the else case is suddenly a real failure mode that needs a real handler.

**How to apply:** when refactoring a helper that combines multiple side effects into a single inline call site, audit the resulting guard branches for any case that now has zero observable effect. If a branch can legitimately be reached at runtime but does nothing, replace it with an explicit `console.warn` for operator triage (or a state-rollback, depending on what "no observable effect" hides). The pattern to look for is the post-refactor `else if (result !== null)` warn that surfaces after a multi-side-effect helper like `reconcileQueue` is inlined and one of its updates is removed.
