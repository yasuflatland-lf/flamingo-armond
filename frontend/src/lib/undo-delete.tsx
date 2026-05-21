"use client";

/**
 * Delayed-DELETE helper with a 5-second undo window.
 *
 * The pending-delete registry is held in a React Context so it is scoped to
 * the Provider instance rather than module scope. Mount <UndoDeleteProvider>
 * once near the app root (e.g. providers.tsx) and consume via useUndoDelete().
 *
 * Behaviour:
 * - The caller performs an optimistic Apollo cache update BEFORE calling
 *   scheduleDelete. The hook does NOT touch the cache itself.
 * - A 5-second timer is started. While the timer is live, a sonner toast is
 *   displayed with an Undo action.
 * - On Undo: timer is cancelled and optimisticRollback is invoked.
 * - On timer elapse: commitDelete is called. On rejection, optimisticRollback
 *   is invoked and onCommitFailed is called with the error.
 * - flushPendingDeletes immediately fires all pending commitDeletes.
 *
 * The Provider also centralises:
 * - beforeunload flush: one listener for the whole app, not per-component.
 * - pathname-change flush: monitors usePathname and flushes on navigation.
 */

import { usePathname } from "next/navigation";
import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
} from "react";
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

export interface ScheduleDeleteOptions {
  /** Unique key for this delete (e.g. the entity ID). A second call with the
   *  same id replaces any existing pending delete for that id. */
  id: string;
  /** Human-readable label for the toast, e.g. "Card deleted". */
  label: string;
  /** Called on Undo click or on commitDelete rejection to restore the
   *  optimistic cache snapshot the caller applied before calling this. */
  optimisticRollback: () => void;
  /** Fires the real DELETE mutation. Called after the undo window elapses. */
  commitDelete: () => Promise<unknown>;
  /** Optional: called with the rejection error when commitDelete rejects.
   *  optimisticRollback is always invoked before this. */
  onCommitFailed?: (err: unknown) => void;
}

/** Return value from scheduleDelete. */
export interface ScheduleDeleteHandle {
  /** Cancel the pending delete and invoke optimisticRollback. */
  undo(): void;
}

/** Public API surface returned by useUndoDelete(). All three methods share the
 *  same pending-delete registry held by the nearest UndoDeleteProvider. */
export interface UndoDeleteAPI {
  scheduleDelete(opts: ScheduleDeleteOptions): ScheduleDeleteHandle;
  flushPendingDeletes(): Promise<void>;
  pendingCount(): number;
}

const UndoDeleteContext = createContext<UndoDeleteAPI | null>(null);

/** Mount once near the app root (e.g. providers.tsx) to make useUndoDelete()
 *  available throughout the tree. The Provider centralises the beforeunload and
 *  pathname-change flush listeners; nesting two providers creates independent
 *  registries, each with its own listeners. */
export function UndoDeleteProvider({ children }: { children: ReactNode }) {
  const pendingRef = useRef<Map<string, PendingDelete>>(new Map());
  const pathname = usePathname();
  const previousPathnameRef = useRef(pathname);

  const flushPendingDeletes = useCallback(async (): Promise<void> => {
    // Snapshot then clear so a concurrent flush cannot double-commit the same entry.
    const entries = Array.from(pendingRef.current.entries());
    for (const [id, entry] of entries) {
      clearTimeout(entry.timerId);
      pendingRef.current.delete(id);
    }
    await Promise.allSettled(
      entries.map(async ([, entry]) => {
        try {
          await entry.commitDelete();
        } catch (commitErr) {
          try {
            entry.optimisticRollback();
          } catch (rollbackErr) {
            console.warn("[undo-delete] optimisticRollback threw during flush", {
              errName: rollbackErr instanceof Error ? rollbackErr.name : "unknown",
            });
          }
          entry.onCommitFailed?.(commitErr);
        }
      }),
    );
  }, []);

  const cancelPending = useCallback((id: string): void => {
    // Does not invoke optimisticRollback — callers are responsible for side effects.
    const entry = pendingRef.current.get(id);
    if (!entry) return;
    clearTimeout(entry.timerId);
    pendingRef.current.delete(id);
  }, []);

  const scheduleDelete = useCallback(
    (opts: ScheduleDeleteOptions): ScheduleDeleteHandle => {
      const { id, label, optimisticRollback, commitDelete, onCommitFailed } = opts;

      if (!id) {
        throw new Error("undo-delete: id must be a non-empty string");
      }

      const existing = pendingRef.current.get(id);
      if (existing !== undefined) {
        console.warn("[undo-delete] re-scheduling pending id; committing prior delete immediately", {
          id,
        });
        if (existing.toastId !== undefined) toast.dismiss(existing.toastId);
        clearTimeout(existing.timerId);
        pendingRef.current.delete(id);
        // onCommitFailed of the prior entry is deliberately NOT called here: the
        // new schedule is the authoritative intent, the item is already removed from
        // the user's view, and surfacing a banner would be misleading with no retry path.
        void existing.commitDelete().catch((err) => {
          console.warn("[undo-delete] prior pending delete commit failed on re-schedule", {
            id,
            errName: err instanceof Error ? err.name : "unknown",
          });
        });
      }

      const commit = async () => {
        pendingRef.current.delete(id);
        try {
          await commitDelete();
        } catch (commitErr) {
          try {
            optimisticRollback();
          } catch (rollbackErr) {
            console.warn("[undo-delete] optimisticRollback threw during commit failure", {
              id,
              errName: rollbackErr instanceof Error ? rollbackErr.name : "unknown",
            });
          }
          onCommitFailed?.(commitErr);
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
      pendingRef.current.set(id, entry);

      return handle;
    },
    [cancelPending],
  );

  // Returns a point-in-time snapshot. Not reactive — the value does not trigger
  // re-renders. Intended for testing and the beforeunload guard.
  const pendingCount = useCallback((): number => {
    return pendingRef.current.size;
  }, []);

  // Flush pending deletes on pathname change so navigating away inside the
  // 5-second undo window does not silently discard a pending DELETE request.
  useEffect(() => {
    if (previousPathnameRef.current === pathname) return;
    const prev = previousPathnameRef.current;
    previousPathnameRef.current = pathname;
    void flushPendingDeletes().catch((err) => {
      console.warn("[UndoDeleteProvider] flushPendingDeletes on pathname change failed", {
        from: prev,
        to: pathname,
        errName: err instanceof Error ? err.name : "unknown",
      });
    });
  }, [pathname, flushPendingDeletes]);

  // Browser-level navigation safety net. Most browsers cancel pending fetch
  // requests when beforeunload fires, so these DELETEs are NOT guaranteed to
  // reach the server. The console.warn surfaces the discard for operators.
  // navigator.sendBeacon is not a viable alternative: it only supports anonymous
  // POSTs and cannot attach the Authorization header this app's GraphQL endpoint requires.
  useEffect(() => {
    function onBeforeUnload() {
      if (pendingRef.current.size === 0) return;
      console.warn(
        "[UndoDeleteProvider] flushPendingDeletes on beforeunload — may be cancelled by browser",
      );
      void flushPendingDeletes();
    }
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [flushPendingDeletes]);

  const api = useMemo(
    () => ({ scheduleDelete, flushPendingDeletes, pendingCount }),
    [scheduleDelete, flushPendingDeletes, pendingCount],
  );

  return <UndoDeleteContext.Provider value={api}>{children}</UndoDeleteContext.Provider>;
}

/** Returns the UndoDeleteAPI from the nearest UndoDeleteProvider.
 *  Throws if called outside a provider tree. */
export function useUndoDelete(): UndoDeleteAPI {
  const ctx = useContext(UndoDeleteContext);
  if (!ctx) throw new Error("useUndoDelete must be called inside <UndoDeleteProvider>");
  return ctx;
}
