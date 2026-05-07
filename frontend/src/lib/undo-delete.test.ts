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

vi.mock("sonner", () => ({
  toast: vi.fn((_label: string, opts?: { action?: { onClick?: () => void } }) => {
    lastUndoAction = opts?.action?.onClick;
  }),
}));

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

    scheduleDelete(optsA);
    scheduleDelete(optsB);

    // Undo only the first delete.
    const undoAAction = lastUndoAction;

    // Schedule B comes second and overwrites lastUndoAction, so capture A's
    // undo explicitly via the returned handle.
    const handleA = scheduleDelete(optsA); // re-schedule A (replaces prior timer)
    handleA.undo();

    await vi.runAllTimersAsync();

    // A was undone — commitDelete should not have been called for A.
    expect(optsA.commitDelete).not.toHaveBeenCalled();
    expect(optsA.optimisticRollback).toHaveBeenCalledOnce();

    // B was not touched — its commitDelete should have fired.
    expect(optsB.commitDelete).toHaveBeenCalledOnce();
    expect(optsB.optimisticRollback).not.toHaveBeenCalled();

    // Suppress unused variable warning.
    void undoAAction;
  });
});

describe("flushPendingDeletes", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    lastUndoAction = undefined;
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
