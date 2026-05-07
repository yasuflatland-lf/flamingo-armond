/**
 * Tests for the delayed-DELETE helper (undo-delete.ts).
 *
 * Uses Vitest fake timers to control the 5-second undo window without
 * real wall-clock delays. The sonner `toast` function is mocked so tests
 * can capture the Undo action callback and invoke it programmatically.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _pendingCount, flushPendingDeletes, scheduleDelete } from "./undo-delete";

// ---------------------------------------------------------------------------
// Mock sonner — tests capture the Undo action callback via `lastUndoAction`.
// ---------------------------------------------------------------------------

let lastUndoAction: (() => void) | undefined;
let toastIdCounter = 0;

// Use vi.hoisted so these refs are available inside the hoisted vi.mock factory.
const { mockToastDismiss } = vi.hoisted(() => ({
  mockToastDismiss: vi.fn(),
}));

vi.mock("sonner", () => {
  const toastFn = vi.fn(
    (_label: string, opts?: { action?: { onClick?: () => void } }) => {
      lastUndoAction = opts?.action?.onClick;
      return ++toastIdCounter;
    },
  ) as ReturnType<typeof vi.fn> & { dismiss: ReturnType<typeof vi.fn> };
  toastFn.dismiss = mockToastDismiss;
  return { toast: toastFn };
});

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeOpts(overrides: Partial<Parameters<typeof scheduleDelete>[0]> = {}) {
  const commitDelete = vi.fn().mockResolvedValue(undefined);
  const optimisticRollback = vi.fn();
  const onCommitFailed = vi.fn();

  return {
    id: "card-1",
    label: "Card deleted",
    commitDelete,
    optimisticRollback,
    onCommitFailed,
    ...overrides,
  } as const;
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

describe("scheduleDelete", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    lastUndoAction = undefined;
    toastIdCounter = 0;
    mockToastDismiss.mockReset();
  });

  afterEach(async () => {
    // Flush any remaining pending deletes to keep the module state clean.
    await vi.runAllTimersAsync();
    vi.useRealTimers();
  });

  it("fires commitDelete after 5 seconds", async () => {
    const opts = makeOpts();
    scheduleDelete(opts);

    expect(opts.commitDelete).not.toHaveBeenCalled();

    // Advance past the undo window.
    await vi.runAllTimersAsync();

    expect(opts.commitDelete).toHaveBeenCalledOnce();
    expect(opts.optimisticRollback).not.toHaveBeenCalled();
  });

  it("undo cancels the timer and invokes optimisticRollback", async () => {
    const opts = makeOpts();
    const handle = scheduleDelete(opts);

    // The toast should have an Undo action.
    expect(lastUndoAction).toBeDefined();

    // Trigger undo programmatically.
    handle.undo();

    // Advance past the window — the timer should have been cancelled.
    await vi.runAllTimersAsync();

    expect(opts.commitDelete).not.toHaveBeenCalled();
    expect(opts.optimisticRollback).toHaveBeenCalledOnce();
  });

  it("undo via the toast action also cancels and rolls back", async () => {
    const opts = makeOpts();
    scheduleDelete(opts);

    // Simulate the user clicking the Undo button in the toast.
    lastUndoAction?.();

    await vi.runAllTimersAsync();

    expect(opts.commitDelete).not.toHaveBeenCalled();
    expect(opts.optimisticRollback).toHaveBeenCalledOnce();
  });

  it("calls optimisticRollback and onCommitFailed when commitDelete rejects", async () => {
    const deleteError = new Error("network error");
    const opts = makeOpts({
      commitDelete: vi.fn().mockRejectedValue(deleteError),
    });

    scheduleDelete(opts);
    await vi.runAllTimersAsync();

    expect(opts.optimisticRollback).toHaveBeenCalledOnce();
    expect(opts.onCommitFailed).toHaveBeenCalledWith(deleteError);
  });

  it("two pending deletes for different ids do not interfere", async () => {
    const optsA = makeOpts({ id: "card-a" });
    const optsB = makeOpts({ id: "card-b" });

    const handleA = scheduleDelete(optsA);
    scheduleDelete(optsB);

    // Undo A via the handle returned from the first schedule.
    handleA.undo();

    await vi.runAllTimersAsync();

    // A was undone — commitDelete should not have been called for A.
    expect(optsA.commitDelete).not.toHaveBeenCalled();
    expect(optsA.optimisticRollback).toHaveBeenCalledOnce();

    // B was not touched — its commitDelete should have fired.
    expect(optsB.commitDelete).toHaveBeenCalledOnce();
    expect(optsB.optimisticRollback).not.toHaveBeenCalled();
  });

  it("throws synchronously when id is an empty string", () => {
    const opts = makeOpts({ id: "" });
    expect(() => scheduleDelete(opts)).toThrow(
      "undo-delete: id must be a non-empty string",
    );
  });

  describe("re-scheduling the same id", () => {
    it("immediately commits the prior pending delete and starts a fresh timer", async () => {
      const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

      const commitA = vi.fn().mockResolvedValue(undefined);
      const rollbackA = vi.fn();
      const failedA = vi.fn();

      const commitB = vi.fn().mockResolvedValue(undefined);
      const rollbackB = vi.fn();

      scheduleDelete({
        id: "dup-id",
        label: "First delete",
        commitDelete: commitA,
        optimisticRollback: rollbackA,
        onCommitFailed: failedA,
      });

      // At this point, the prior delete is pending — not yet committed.
      expect(commitA).not.toHaveBeenCalled();

      // Re-schedule the same id.
      scheduleDelete({
        id: "dup-id",
        label: "Second delete",
        commitDelete: commitB,
        optimisticRollback: rollbackB,
      });

      // The prior commitDelete must have fired immediately (no rollback, since
      // the optimistic remove is what the caller committed to UX-wise).
      // Allow the promise microtask to flush via a resolved-promise await.
      await Promise.resolve();
      expect(commitA).toHaveBeenCalledOnce();
      expect(rollbackA).not.toHaveBeenCalled();

      // A console.warn must have been emitted with a discriminating `id` key.
      expect(warnSpy).toHaveBeenCalledWith(
        "[undo-delete] re-scheduling pending id; committing prior delete immediately",
        expect.objectContaining({ id: "dup-id" }),
      );

      // The new schedule's commit fires after 5 s.
      expect(commitB).not.toHaveBeenCalled();
      await vi.runAllTimersAsync();
      expect(commitB).toHaveBeenCalledOnce();
      expect(rollbackB).not.toHaveBeenCalled();

      warnSpy.mockRestore();
    });

    it("invokes the prior onCommitFailed when the prior commitDelete rejects on re-schedule", async () => {
      const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

      const priorError = new Error("prior commit failed");
      const commitA = vi.fn().mockRejectedValue(priorError);
      const rollbackA = vi.fn();
      const failedA = vi.fn();

      scheduleDelete({
        id: "dup-reject",
        label: "First delete",
        commitDelete: commitA,
        optimisticRollback: rollbackA,
        onCommitFailed: failedA,
      });

      // Re-schedule — triggers immediate commit of the prior entry.
      scheduleDelete({
        id: "dup-reject",
        label: "Second delete",
        commitDelete: vi.fn().mockResolvedValue(undefined),
        optimisticRollback: vi.fn(),
      });

      // Flush microtasks so the rejected promise settles.
      await Promise.resolve();

      // The prior commitDelete was called and rejected; onCommitFailed must fire.
      expect(commitA).toHaveBeenCalledOnce();
      // Note: optimisticRollback is NOT called on re-schedule commit path —
      // the prior optimistic remove is still what the caller wanted.
      expect(rollbackA).not.toHaveBeenCalled();
      expect(failedA).toHaveBeenCalledWith(priorError);

      // Clean up second timer.
      await vi.runAllTimersAsync();
      warnSpy.mockRestore();
    });
  });
});

describe("flushPendingDeletes", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    lastUndoAction = undefined;
    toastIdCounter = 0;
    mockToastDismiss.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("immediately fires all pending commitDeletes without waiting for the timer", async () => {
    const optsA = makeOpts({ id: "flush-a" });
    const optsB = makeOpts({ id: "flush-b" });

    scheduleDelete(optsA);
    scheduleDelete(optsB);

    expect(_pendingCount()).toBe(2);

    // Flush before any timer fires.
    await flushPendingDeletes();

    expect(optsA.commitDelete).toHaveBeenCalledOnce();
    expect(optsB.commitDelete).toHaveBeenCalledOnce();
    expect(_pendingCount()).toBe(0);

    // No more timers left — running all timers should not fire anything extra.
    await vi.runAllTimersAsync();

    expect(optsA.commitDelete).toHaveBeenCalledOnce();
    expect(optsB.commitDelete).toHaveBeenCalledOnce();
  });

  it("calls optimisticRollback for any commitDelete that rejects during flush", async () => {
    const deleteError = new Error("flush commit failure");
    const opts = makeOpts({
      id: "flush-fail",
      commitDelete: vi.fn().mockRejectedValue(deleteError),
    });

    scheduleDelete(opts);
    await flushPendingDeletes();

    expect(opts.optimisticRollback).toHaveBeenCalledOnce();
    expect(opts.onCommitFailed).toHaveBeenCalledWith(deleteError);
  });

  it("concurrent flush calls do not double-commit the same item", async () => {
    const opts = makeOpts({ id: "flush-once" });
    scheduleDelete(opts);

    // Fire two flushes concurrently.
    await Promise.all([flushPendingDeletes(), flushPendingDeletes()]);

    // commitDelete must have been called exactly once.
    expect(opts.commitDelete).toHaveBeenCalledOnce();
  });
});
