# Refetch after a mutation success: isolate it from the error-classification catch

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

A mutation hook that refetches a connection on success — e.g. `useMergeFromCatalog`, which copies a master deck into an existing cardgroup and then refreshes that cardgroup's cards list — has two refetch footguns. Both turn a *succeeded* server operation into a wrong client outcome.

## 1. The post-success refetch must have its own `try/catch`

`await apollo.refetchQueries(...)` placed inside the same `try` block that classifies the **mutation's** errors will route a refetch transport failure into that mutation `catch`. The classifier sees a plain `Error` (not a `CombinedGraphQLErrors`), maps it to a generic failure, and the hook returns `{ status: "rejected" }` — so the UI shows "merge failed, please try again" even though the server already merged. The user then re-merges, producing a duplicate operation.

```ts
// AVOID: a refetch failure is misclassified as a mutation failure.
try {
  const result = await mergeMasterCardgroup({ variables });
  const payload = result.data?.mergeMasterCardgroup;
  if (payload?.__typename === "MergeMasterCardgroupSuccess") {
    await apollo.refetchQueries({ include: [CardsByCardgroupConnectionDocument], onQueryUpdated });
    return { status: "success", addedCount: payload.addedCount, updatedCount: payload.updatedCount };
  }
  // ...
} catch (err) {
  // A rejected refetch lands here too, and gets classified as "rejected".
  return classifyToAuthOutcome(err, "useMergeFromCatalog", "mergeMasterCardgroup", { ... });
}

// PREFER: the merge succeeded once the success variant is confirmed — the cache
// refresh is best-effort and must not downgrade the outcome.
if (payload?.__typename === "MergeMasterCardgroupSuccess") {
  try {
    await apollo.refetchQueries({ include: [CardsByCardgroupConnectionDocument], onQueryUpdated });
  } catch (refetchErr) {
    // Merge already succeeded on the server; only the post-success cache
    // refresh failed. The cards list may be stale until the next navigation.
    // Do NOT log refetchErr.message — backend messages may carry user content.
    console.warn("[useMergeFromCatalog] refetch after successful merge failed", {
      targetCardgroupId,
      name: refetchErr instanceof Error ? refetchErr.name : "unknown",
    });
  }
  return { status: "success", addedCount: payload.addedCount, updatedCount: payload.updatedCount };
}
```

## 2. The `onQueryUpdated` predicate matches on identity only, and `return true`

`refetchQueries({ onQueryUpdated })` decides per cached query whether to refetch it. Match only on the query's **stable identity** (here `cardgroupId`) and `return true`, which tells Apollo to refetch that query with **its own current variables**. Comparing the live query's variables against a default-vars factory over-constrains the predicate: `cardsDefaultVars(id)` always carries `search: null`, so the equality check `variables.search !== refetchVariables.search` is `true` whenever the user has typed a card-search term, and the refetch is silently skipped — the merged cards never appear until a reload.

```ts
// AVOID: over-constrained predicate skips the refetch under an active search filter.
onQueryUpdated: (observableQuery) => {
  const variables = observableQuery.options.variables as CardsByCardgroupConnectionQueryVariables | undefined;
  if (
    !variables ||
    variables.cardgroupId !== refetchVariables.cardgroupId ||
    variables.first !== refetchVariables.first ||
    variables.search !== refetchVariables.search // refetchVariables.search is always null
  ) {
    return false;
  }
  return observableQuery.refetch(refetchVariables); // also forces search back to null
},

// PREFER: match the identity key only; return true to refetch with live variables.
onQueryUpdated: (observableQuery) => {
  const variables = observableQuery.options.variables as CardsByCardgroupConnectionQueryVariables | undefined;
  if (!variables || variables.cardgroupId !== refetchVariables.cardgroupId) {
    return false;
  }
  // Refetch with the query's own current variables so an active search / page is preserved.
  return true;
},
```

**Why:** a mutation hook that conflates "the mutation failed" with "the post-success cache refresh failed" reports a false failure for a succeeded operation, and the user's natural reaction (retry) re-runs the mutation. An over-constrained refetch predicate silently skips the refresh under a filter, leaving a stale list with no error — the worst kind of silent failure because the operation *did* work, only the view did not update.

**How to apply:** in any hook that refetches after a mutation success, (a) confirm the success variant first, then wrap the refetch in its own `try/catch` that logs `err.name` only and returns the success outcome regardless; and (b) build the `onQueryUpdated` predicate from the **identity** of the target connection (the FK / id the mutation touched), never from a full default-vars object, and `return true` rather than `observableQuery.refetch(defaultVars)` so the user's active variables survive. Test both: a success-with-rejected-refetch case that still returns `success`, and a predicate unit-check that returns `true` for the matching id and `false` otherwise. Reference: `frontend/src/app/cardgroups/[id]/use-merge-from-catalog.ts`. Related: [fire-and-forget mutation warn-on-null-and-non-success](fire-and-forget-mutation-warn-on-null-and-non-success.md) (the structured-warn shape) and [pair every `try { ... } finally` with a `catch`](pair-try-finally-with-catch-for-transport.md) (the transport-rejection channel).
