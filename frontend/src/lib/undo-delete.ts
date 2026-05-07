/**
 * Delayed-DELETE helper with a 5-second undo window.
 *
 * Behaviour:
 * - The caller performs an optimistic Apollo cache update BEFORE calling
 *   `scheduleDelete`. The helper does NOT touch the cache itself.
 * - A 5-second timer is started. While the timer is live, a sonner toast is
 *   displayed with an Undo action.
 * - On Undo: timer is cancelled and `optimisticRollback` is invoked so the
 *   caller can restore the cache snapshot.
 * - On timer elapse: `commitDelete` is called. If it rejects,
 *   `optimisticRollback` is invoked (to un-remove the item from the cache) and
 *   `onCommitFailed` is called with the error.
 * - `flushPendingDeletes` immediately fires all pending `commitDelete`s and
 *   returns a Promise that resolves when they all settle. Used on pathname
 *   change and `beforeunload` so navigating away cannot silently discard a
 *   pending delete.
 *
 * This module is intentionally framework-agnostic — it does not import Apollo
 * Client or any React hooks. Callers supply closures for all framework
 * interactions.
 */

import { toast } from "sonner";

/** Duration of the undo window in milliseconds. */
const UNDO_DELAY_MS = 5000;

interface PendingDelete {
  timerId: ReturnType<typeof setTimeout>;
  toastId: string | number | undefined;
  commitDelete: () => Promise<unknown>;
  optimisticRollback: () => void;
  onCommitFailed?: (err: unknown) => void;
}

/** Module-level registry: id → pending timer state. */
const pending = new Map<string, PendingDelete>();

export interface ScheduleDeleteOptions {
  /** Unique key for this delete (e.g. the entity ID). A second call with the
   *  same id replaces any existing pending delete for that id. */
  id: string;
  /** Human-readable label for the toast, e.g. "Card deleted". */
  label: string;
  /** Called on Undo click or on `commitDelete` rejection to restore the
   *  optimistic cache snapshot the caller applied before calling this. */
  optimisticRollback: () => void;
  /** Fires the real DELETE mutation. Called after the undo window elapses. */
  commitDelete: () => Promise<unknown>;
  /** Optional: called with the rejection error when `commitDelete` rejects.
   *  `optimisticRollback` is always invoked before this. */
  onCommitFailed?: (err: unknown) => void;
}

/** Return value from `scheduleDelete`. */
export interface ScheduleDeleteHandle {
  /** Cancel the pending delete and invoke `optimisticRollback`. */
  undo(): void;
}

/**
 * Schedule a DELETE to be committed after a 5-second undo window.
 *
 * The caller must have already applied an optimistic cache update before
 * calling this function.
 *
 * @returns A handle whose `undo()` method cancels the scheduled delete and
 *          restores the cache via `optimisticRollback`.
 */
export function scheduleDelete(opts: ScheduleDeleteOptions): ScheduleDeleteHandle {
  const { id, label, optimisticRollback, commitDelete, onCommitFailed } = opts;

  // Empty id would let two callers silently cancel each other's timers.
  if (!id) {
    throw new Error("undo-delete: id must be a non-empty string");
  }

  // If a previous pending delete for the same id already exists, commit it
  // immediately so the optimistic remove already applied to the cache is backed
  // by a real DELETE before the new schedule starts.
  const existing = pending.get(id);
  if (existing !== undefined) {
    console.warn("[undo-delete] re-scheduling pending id; committing prior delete immediately", {
      id,
    });
    if (existing.toastId !== undefined) toast.dismiss(existing.toastId);
    clearTimeout(existing.timerId);
    pending.delete(id);
    void existing.commitDelete().catch((err) => {
      // Prior optimistic-remove is preserved (the new schedule is the
      // authoritative intent). Do NOT call existing.onCommitFailed here
      // because the item is already gone from the user's view; surfacing
      // a banner would be misleading and the user has no actionable retry path.
      console.warn("[undo-delete] prior pending delete commit failed on re-schedule", { id, err });
    });
  }

  const commit = async () => {
    pending.delete(id);
    try {
      await commitDelete();
    } catch (err) {
      optimisticRollback();
      onCommitFailed?.(err);
    }
  };

  const timerId = setTimeout(commit, UNDO_DELAY_MS);

  const handle: ScheduleDeleteHandle = {
    undo() {
      cancelPending(id);
      optimisticRollback();
    },
  };

  const toastId = toast(label, {
    duration: UNDO_DELAY_MS,
    action: {
      label: "Undo",
      onClick: () => handle.undo(),
    },
  });

  const entry: PendingDelete = {
    timerId,
    toastId,
    commitDelete,
    optimisticRollback,
    onCommitFailed,
  };
  pending.set(id, entry);

  return handle;
}

/**
 * Immediately fire all pending `commitDelete` callbacks and return a Promise
 * that resolves when they all settle (fulfilled or rejected).
 *
 * Call this on pathname change and on the `beforeunload` event to ensure
 * pending deletes are not silently discarded when the user navigates away.
 */
export async function flushPendingDeletes(): Promise<void> {
  // Snapshot the current pending entries before clearing the map.
  const entries = Array.from(pending.entries());

  // Clear the map and cancel all timers immediately so a concurrent flush
  // cannot double-commit.
  for (const [id, entry] of entries) {
    clearTimeout(entry.timerId);
    pending.delete(id);
  }

  // Fire all commits in parallel; each handles its own rollback on failure.
  await Promise.allSettled(
    entries.map(async ([, entry]) => {
      try {
        await entry.commitDelete();
      } catch (err) {
        entry.optimisticRollback();
        entry.onCommitFailed?.(err);
      }
    }),
  );
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/**
 * Cancel and remove the pending entry for `id` without committing or rolling
 * back. The caller is responsible for any side effects.
 */
function cancelPending(id: string): void {
  const entry = pending.get(id);
  if (!entry) return;
  clearTimeout(entry.timerId);
  pending.delete(id);
}

/**
 * Returns the number of currently pending deletes. Exposed for testing only.
 * @internal
 */
export function _pendingCount(): number {
  return pending.size;
}
