# Frontend: delayed-DELETE undo toast, SwipeableRow, and useReducedMotion

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters. Covers the sonner snackbar undo pattern, the mobile swipe-to-delete row component, and the `useReducedMotion` JS hook.

## Delayed-DELETE with sonner snackbar undo

### Why

Replace per-row `AlertDialog` confirms with an optimistic remove + 5-second delayed commit. The user never waits for a server round-trip on the success path; Undo cancels the timer locally without a server-side restore mutation. Bulk delete retains the `AlertDialog` confirm because rolling back N concurrent optimistic removes is disproportionately complex (see [Bulk delete keeps AlertDialog](#bulk-delete-keeps-alertdialog)).

### Architecture

**Toaster mount point.** `<Toaster richColors closeButton />` (from `frontend/src/components/ui/sonner.tsx`) mounts inside `<AppShell>` in `frontend/src/app/layout.tsx`. The placement is load-bearing: sonner requires a client rendering context, so the `Toaster` cannot live at the RSC layout level. `AppShell` is already the client-boundary component, so every authenticated route gets the toast container without an extra wrapper.

**`scheduleDelete` helper.** `frontend/src/lib/undo-delete.ts` exports a framework-agnostic singleton:

```ts
scheduleDelete({
  id,             // unique key (e.g. entity UUID) — required, non-empty
  label,          // toast body text, e.g. "Card deleted"
  optimisticRollback,  // restores the caller's prior Apollo cache snapshot
  commitDelete,   // fires the real DELETE mutation; returns Promise<unknown>
  onCommitFailed, // optional; called with the rejection error after rollback
}): ScheduleDeleteHandle  // .undo() cancels timer + calls optimisticRollback
```

The module also exports `flushPendingDeletes(): Promise<void>` and the test-only `_pendingCount()` export.

**Caller responsibility — sequencing:**

1. Apply the optimistic Apollo cache update (remove the edge, decrement `totalCount`) using `readQuery + writeQuery`. See [`.claude/rules/pagination.md` § "Frontend cache patterns"](../../.claude/rules/pagination.md#frontend-cache-patterns) for the "Connection delete" cache pattern.
2. Call `scheduleDelete(...)` with the rollback and commit closures. The helper does not touch the cache itself.
3. On Undo click, `optimisticRollback` is invoked to restore the snapshot.
4. On timer elapse, `commitDelete()` runs. Rejection triggers `optimisticRollback` then `onCommitFailed`.

**Re-schedule guard.** A second call with the same `id` before the first timer elapses immediately commits the prior delete (to avoid leaving a dangling optimistic remove), then starts a new timer. A `console.warn` fires for operator triage.

**Navigation flush.** Call `flushPendingDeletes()` on pathname change and in a `beforeunload` listener so navigating away cannot silently drop a pending commit. The flush fires all pending `commitDelete`s immediately and returns a `Promise` that settles when they all complete. Mount the flush in a `useEffect` keyed on `pathname`:

```ts
useEffect(() => {
  return () => { void flushPendingDeletes(); };
}, [pathname]);
```

The `return () =>` cleanup form avoids the async-cleanup pitfall: a raw `useEffect(async () => ...)` body would need an inner IIFE and its returned `Promise` is discarded by React. The cleanup function is synchronous; `flushPendingDeletes` itself is async and runs fire-and-forget in that scope, which is intentional — the effect's lifecycle is already over.

### Bulk delete keeps AlertDialog

Rolling back N concurrent optimistic removes (one per selected card) is more complex than the single-row case: the pre-delete snapshot grows with selection size and concurrent bulk + per-row interleaving creates hard-to-reason-about states. The cost/benefit favours an `AlertDialog` confirm for bulk delete paths.

---

## SwipeableRow for mobile swipe-to-delete

### Why

Mobile users expect a swipe gesture to reveal and commit a delete. Desktop uses a hover-visible Delete icon. `SwipeableRow` provides the mobile path without conflating it with the desktop affordance.

### Props and imperative handle

```ts
// frontend/src/components/cardgroups/swipeable-row.tsx
interface SwipeableRowProps {
  children: React.ReactNode;
  onDelete: () => void;
  disabled?: boolean;
  /**
   * Required (`string | null`) — pass null to accept the default "Delete" label,
   * or a contextual string (e.g. "Delete card") to override it.
   * See docs/frontend/typescript-conventions.md § "Required string | null
   * over optional ?: string | null".
   */
  ariaLabel: string | null;
}

interface SwipeableRowHandle {
  close(): void; // snap back to resting position
}
```

### Gesture thresholds

| Release fraction | Outcome |
|---|---|
| ≥ 60% of row width | Row slides off screen; `onDelete()` fires |
| 30%–60% | Snaps to half-open; Delete button revealed |
| < 30% or vertical | Snaps back |

Vertical scroll is never intercepted — the component detects horizontal vs. vertical movement and hands vertical gestures to the browser.

### `disabled` and reduced-motion behaviour

- `disabled={true}` (e.g. when `selectedIds.size > 0` or the row is being edited) turns off the gesture entirely so selection-mode checkboxes receive touch events without interference.
- When `prefers-reduced-motion: reduce` is active, `SwipeableRow` renders its `children` directly (no animated wrapper). The parent is responsible for a visible delete affordance in that case (e.g. making the hover Delete icon always-visible). See [useReducedMotion](#usereducedmotion-js-hook).

### Parent responsibility: close other rows on activation

Maintain a `Map<id, RefObject<SwipeableRowHandle>>` and call `.close()` on every other row when a row is tapped-to-edit or when the user interacts with a different row:

```ts
rowRefs.forEach((ref, rowId) => {
  if (rowId !== activatedId) ref.current?.close();
});
```

---

## `useReducedMotion` JS hook

`frontend/src/lib/use-reduced-motion.ts` exports:

```ts
function useReducedMotion(): boolean
```

Returns `true` when `(prefers-reduced-motion: reduce)` is active; updates reactively via a `MediaQueryList` `change` listener so a user who toggles the OS setting mid-session sees the effect immediately. Returns `false` in SSR environments where `window` is not defined.

### Why a JS hook over Tailwind `motion-reduce:` classes

CSS `motion-reduce:` classes suppress visual transitions but cannot disable gesture handlers or autoclose timers — both of which are non-visual side effects. The hook lets `SwipeableRow` skip the animated layer entirely (not just restyle it), and downstream components can read the same reactive boolean to disable autoclose timers or animated scroll-into-view calls.

### How to apply

Any motion-driven interaction component with a non-visual side effect (gesture commit, autoclose timer, `scrollIntoView`) should import `useReducedMotion` and branch on its return value. Pure-visual animation (fade-in, slide-in) can use either the hook or Tailwind's `motion-reduce:` class; prefer the Tailwind class when no non-visual side effect exists, to keep the component free of a hook dependency.
