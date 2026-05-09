# Re-scheduling a module-level singleton entry: commit prior immediately; warn-only on prior failure

> Part of [`.claude/rules/frontend-typescript-conventions.md`](../frontend-typescript-conventions.md). See the index for related rules.

When the same entity id is re-scheduled on a module-level pending registry (e.g. a fast double-swipe-to-delete on the same row triggers `scheduleDelete` twice for the same card id), naively calling `cancelPending(id)` discards the prior entry without invoking its `commitDelete`. The optimistic cache removal is now permanent — but the server never received the DELETE. Equally bad: routing the prior commit's rejection to `onCommitFailed` shows a user-facing "Could not delete. Please try again." banner for an item already gone from view. The user has no actionable retry path; the banner is misleading.

The correct pattern has two parts: (1) commit the prior entry immediately — preserving the user's intent that the prior optimistic remove is permanent — and (2) route the prior commit's rejection to `console.warn` only, NOT to `onCommitFailed`:

```ts
const existing = pending.get(opts.id);
if (existing !== undefined) {
  if (existing.toastId !== undefined) toast.dismiss(existing.toastId);
  clearTimeout(existing.timerId);
  pending.delete(opts.id);
  void existing.commitDelete().catch((err) => {
    // Prior optimistic-remove is preserved (the new schedule is the authoritative
    // intent). Do NOT call existing.onCommitFailed — the item is already gone
    // from the user's view; a banner would be misleading and has no retry path.
    console.warn(
      "[undo-delete] prior pending delete commit failed on re-schedule",
      { id: opts.id, err },
    );
  });
}
```

Per § "`expect.objectContaining({ message })` is not enough — add a discriminating key": include `id` and `err` in the structured warn payload so any test matcher discriminates against a bare `Error` regression.

**Why:** the trade-off is intentional. A server-DELETE failure on the prior entry leaves the client cache and server briefly out of sync — surfaced via a developer-tools warn. The user does NOT see a retry-prompted banner because there is no valid retry path for an entity the user has already re-deleted.

**How to apply:** any module-level pending registry that accepts a re-schedule for an already-pending id must (a) dismiss the prior UI affordance (toast, banner), (b) clear the prior timer, (c) fire the prior `commitDelete` immediately, and (d) route the prior commit's rejection to `console.warn` with a structured `{ id, err }` payload — NOT to the user-facing error callback. Reference: `frontend/src/lib/undo-delete.ts` `scheduleDelete` re-schedule branch.
