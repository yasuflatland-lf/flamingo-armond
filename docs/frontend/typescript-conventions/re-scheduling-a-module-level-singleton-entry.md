# Re-scheduling a module-level singleton entry: commit prior immediately; warn-only on prior failure

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

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

## Timer lifecycle checklist

Every timer that drives a deferred commit must be cancelled at three distinct points. Missing any one of them causes the timer to fire redundantly, double-commit, or update stale state after unmount.

1. **Cancel before rescheduling.** When `scheduleX(id)` is called while a prior timer for the same id is already pending, `clearTimeout` the prior handle before setting a new one. (This is the re-schedule branch described above.)

2. **Cancel on unmount.** A `useEffect` that creates a timer must return a cleanup function that calls `clearTimeout`. Use `[]` deps (or the correct deps array) so the cleanup runs exactly once, when the component unmounts. Forgetting this causes the `setTimeout` body to run against an unmounted component, which may call `setState` on an already-unmounted tree or dispatch a mutation after the owning route has been torn down.

   ```ts
   useEffect(() => {
     const id = setTimeout(() => doWork(), delay);
     return () => clearTimeout(id);
   }, []);
   ```

3. **Cancel when a parallel synchronous path commits the same work.** When a gesture handler (e.g. a swipe-confirm callback) performs the same commit synchronously, it must also `clearTimeout` the programmatic timer that would have committed later. Without this, the programmatic timer fires after the gesture has already committed, producing a double-commit or a redundant network call.

   ```ts
   // Gesture path — commits immediately and cancels the scheduled timer
   function onConfirmGesture(id: string) {
     const entry = pending.get(id);
     if (entry) {
       clearTimeout(entry.timerId);   // cancel the programmatic path
       pending.delete(id);
       void entry.commitDelete();
     }
   }
   ```

## Defense-in-depth: id-guard inside the setTimeout body

Even when the gesture path cancels the programmatic timer via `clearTimeout`, a race exists: the JavaScript event loop can have already queued the `setTimeout` callback at the instant `clearTimeout` is called. The callback then runs after the gesture commit, and if it blindly re-commits it creates a duplicate mutation.

Guard the `setTimeout` body by capturing the entity id at schedule time as a "token" and comparing it against the currently-active token immediately before the commit fires:

```ts
function scheduleDelete(id: string) {
  const timerId = setTimeout(() => {
    // Defense-in-depth: verify this timer is still the active one for this id.
    // The gesture path may have cleared the entry already (clearTimeout fires
    // asynchronously in some environments, or a microtask may have run first).
    if (!pending.has(id)) return;   // gesture already committed — do nothing
    const entry = pending.get(id)!;
    pending.delete(id);
    void entry.commitDelete().catch(/* ... */);
  }, DELAY_MS);
  pending.set(id, { timerId, commitDelete, /* ... */ });
}
```

The two layers form a defense-in-depth pair:

- **Layer A (gesture path):** `clearTimeout(timerId)` — prevents the callback from being scheduled in the normal case.
- **Layer B (id-guard in setTimeout body):** `if (!pending.has(id)) return` — a fallback that handles the race where the event loop already queued the callback before `clearTimeout` ran.

Reference implementation: `frontend/src/components/learn/swipe-card-stack.tsx` applies both layers — the gesture handler cancels the programmatic timer (Layer A) and the `setTimeout` body checks whether the entry is still current before committing (Layer B).
