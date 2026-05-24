# Preserve a conflict banner across a same-entity auto-refetch

> Part of the [frontend TypeScript conventions](../typescript-conventions.md).

## The problem

A typed-error outcome (e.g. an optimistic-concurrency `ConcurrentUpdateError`) is handled by showing a banner **and** auto-triggering a refetch of the same entity so the form shows the latest server values. When the refetch resolves, the refreshed entity object swaps into the form-sync `useEffect` that resets the inputs — and that effect also clears the error banner. The admin is left with their edits silently replaced and no surviving explanation of why.

Two Apollo subtleties make a naive fix fail:

1. **`useFragment` returns a new object reference on every cache broadcast**, even when the underlying data is unchanged. So the form-sync effect (keyed on the entity object identity) can fire more than once per logical change.
2. **A conflict reload writes the same normalized entity from more than one query.** The parent reload runs both a list `refetch()` and a detail `loadAdminUser()`, and both write `User:<id>`. The same-id object therefore swaps **two or more times**. A single-shot "re-show the banner once, then clear a flag" ref survives the first swap but the second swap clears the banner again.

## The fix: track the last-synced id, not a one-shot flag

Keep a `useRef` of the id the form last synced to. Clear the banner only when the synced id **changes** (the user switched to a different entity); preserve it across **same-id** reloads. Reset the ref to `null` on close so reopening the same entity counts as a fresh sync and clears any leftover banner.

```tsx
const lastSyncedId = useRef<string | null>(null);

useEffect(() => {
  if (!user) {
    lastSyncedId.current = null; // reopen-same-user is then a fresh sync
    return;
  }
  setDisplayName(user.displayName ?? "");
  // ...reset other fields...
  if (lastSyncedId.current !== user.id) {
    setSaveError(""); // different user → drop the stale banner
  }
  // same-id reload (the conflict refresh) → keep the banner
  lastSyncedId.current = user.id;
}, [user, resetEdit]);
```

This is the "auto-reload + keep banner" UX for an optimistic-concurrency conflict: the form shows the latest server values, the refreshed `version` token makes a re-save succeed, and the explanation stays visible. Pair it with the [`.claude/rules/pagination.md`](../../../.claude/rules/pagination.md) rule "drop `optimisticResponse` for mutations that can fail with typed GraphQL errors" — a concurrency-failing mutation must not carry an optimistic write.

Reference: `frontend/src/app/admin/users/admin-user-profile-sheet.tsx` (`lastSyncedId`), `frontend/src/app/admin/users/admin-users-client.tsx` (`reloadEditedUser`). Backend side: [Optimistic version concurrency for cross-request edits](../../backend/library-gotchas/optimistic-version-concurrency-cross-request.md).

## Testing notes

- **Prove survival across *multiple* swaps.** A single rerender with a bumped-version same-id object would pass even with the broken single-shot flag. Rerender **at least twice** with same-id, fresh-value objects and assert the banner persists after each — that is what distinguishes the last-synced-id approach from the one-shot flag.
- **Shared-entity race vs. typed input in `MockedProvider`.** When both a list query and a detail query write the same normalized entity, a late cache broadcast re-fires the form-sync effect and resets an input mid-`userEvent.type` (producing e.g. `"AliceAlice 2"`). Isolate the writer: empty the sibling query so only one query writes the entity, or trigger the mutation without editing the form (an unchanged save still fires the mutation).
- **`findByRole`, not `getByRole`, for a late-settling sibling query.** When a second query (e.g. roles) settles a tick after the primary query (e.g. the user), a synchronous `getByRole` races it and flakes under load. Await the dependent element with `findByRole`.
