// @vitest-environment jsdom

/**
 * Tests for UndoDeleteProvider and useUndoDelete hook.
 *
 * Uses Vitest fake timers to control the 5-second undo window without
 * real wall-clock delays. The sonner `toast` function is mocked so tests
 * can capture the Undo action callback and invoke it programmatically.
 */

import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { UndoDeleteProvider, useUndoDelete } from "./undo-delete";

// ---------------------------------------------------------------------------
// Mock next/navigation — Provider uses usePathname internally.
// ---------------------------------------------------------------------------

vi.mock("next/navigation", () => ({
  usePathname: () => "/test",
}));

// ---------------------------------------------------------------------------
// Mock sonner — tests capture the Undo action callback via lastUndoAction.
// ---------------------------------------------------------------------------

let lastUndoAction: (() => void) | undefined;
let toastIdCounter = 0;

const { mockToastDismiss } = vi.hoisted(() => ({
  mockToastDismiss: vi.fn(),
}));

vi.mock("sonner", () => {
  const toastFn = vi.fn((_label: string, opts?: { action?: { onClick?: () => void } }) => {
    lastUndoAction = opts?.action?.onClick;
    return ++toastIdCounter;
  }) as ReturnType<typeof vi.fn> & { dismiss: ReturnType<typeof vi.fn> };
  toastFn.dismiss = mockToastDismiss;
  return { toast: toastFn };
});

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function wrapper({ children }: { children: ReactNode }) {
  return <UndoDeleteProvider>{children}</UndoDeleteProvider>;
}

function makeOpts(
  overrides: Partial<Parameters<ReturnType<typeof useUndoDelete>["scheduleDelete"]>[0]> = {},
) {
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

describe("useUndoDelete — scheduleDelete", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    lastUndoAction = undefined;
    toastIdCounter = 0;
    mockToastDismiss.mockReset();
  });

  afterEach(async () => {
    await act(async () => {
      await vi.runAllTimersAsync();
    });
    vi.useRealTimers();
  });

  it("fires commitDelete after 5 seconds", async () => {
    const { result } = renderHook(useUndoDelete, { wrapper });
    const opts = makeOpts();

    act(() => {
      result.current.scheduleDelete(opts);
    });

    expect(opts.commitDelete).not.toHaveBeenCalled();

    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(opts.commitDelete).toHaveBeenCalledOnce();
    expect(opts.optimisticRollback).not.toHaveBeenCalled();
  });

  it("undo cancels the timer and invokes optimisticRollback", async () => {
    const { result } = renderHook(useUndoDelete, { wrapper });
    const opts = makeOpts();

    let handle: ReturnType<typeof result.current.scheduleDelete>;
    act(() => {
      handle = result.current.scheduleDelete(opts);
    });

    expect(lastUndoAction).toBeDefined();

    act(() => {
      handle?.undo();
    });

    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(opts.commitDelete).not.toHaveBeenCalled();
    expect(opts.optimisticRollback).toHaveBeenCalledOnce();
  });

  it("undo via the toast action also cancels and rolls back", async () => {
    const { result } = renderHook(useUndoDelete, { wrapper });
    const opts = makeOpts();

    act(() => {
      result.current.scheduleDelete(opts);
    });

    act(() => {
      lastUndoAction?.();
    });

    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(opts.commitDelete).not.toHaveBeenCalled();
    expect(opts.optimisticRollback).toHaveBeenCalledOnce();
  });

  it("calls optimisticRollback and onCommitFailed when commitDelete rejects", async () => {
    const deleteError = new Error("network error");
    const { result } = renderHook(useUndoDelete, { wrapper });
    const opts = makeOpts({
      commitDelete: vi.fn().mockRejectedValue(deleteError),
    });

    act(() => {
      result.current.scheduleDelete(opts);
    });

    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(opts.optimisticRollback).toHaveBeenCalledOnce();
    expect(opts.onCommitFailed).toHaveBeenCalledWith(deleteError);
  });

  it("two pending deletes for different ids do not interfere", async () => {
    const { result } = renderHook(useUndoDelete, { wrapper });
    const optsA = makeOpts({ id: "card-a" });
    const optsB = makeOpts({ id: "card-b" });

    let handleA: ReturnType<typeof result.current.scheduleDelete>;
    act(() => {
      handleA = result.current.scheduleDelete(optsA);
      result.current.scheduleDelete(optsB);
    });

    act(() => {
      handleA?.undo();
    });

    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(optsA.commitDelete).not.toHaveBeenCalled();
    expect(optsA.optimisticRollback).toHaveBeenCalledOnce();

    expect(optsB.commitDelete).toHaveBeenCalledOnce();
    expect(optsB.optimisticRollback).not.toHaveBeenCalled();
  });

  it("throws synchronously when id is an empty string", () => {
    const { result } = renderHook(useUndoDelete, { wrapper });
    const opts = makeOpts({ id: "" });
    expect(() => {
      act(() => {
        result.current.scheduleDelete(opts);
      });
    }).toThrow("undo-delete: id must be a non-empty string");
  });

  describe("re-scheduling the same id", () => {
    it("immediately commits the prior pending delete and starts a fresh timer", async () => {
      const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
      const { result } = renderHook(useUndoDelete, { wrapper });

      const commitA = vi.fn().mockResolvedValue(undefined);
      const rollbackA = vi.fn();

      const commitB = vi.fn().mockResolvedValue(undefined);
      const rollbackB = vi.fn();

      act(() => {
        result.current.scheduleDelete({
          id: "dup-id",
          label: "First delete",
          commitDelete: commitA,
          optimisticRollback: rollbackA,
        });
      });

      expect(commitA).not.toHaveBeenCalled();

      act(() => {
        result.current.scheduleDelete({
          id: "dup-id",
          label: "Second delete",
          commitDelete: commitB,
          optimisticRollback: rollbackB,
        });
      });

      await act(async () => {
        await Promise.resolve();
      });

      expect(commitA).toHaveBeenCalledOnce();
      // The prior entry's toast must be dismissed when the re-schedule fires.
      expect(mockToastDismiss).toHaveBeenCalledWith(1);
      expect(rollbackA).not.toHaveBeenCalled();

      expect(warnSpy).toHaveBeenCalledWith(
        "[undo-delete] re-scheduling pending id; committing prior delete immediately",
        expect.objectContaining({ id: "dup-id" }),
      );

      expect(commitB).not.toHaveBeenCalled();

      await act(async () => {
        await vi.runAllTimersAsync();
      });

      expect(commitB).toHaveBeenCalledOnce();
      expect(rollbackB).not.toHaveBeenCalled();

      warnSpy.mockRestore();
    });

    it("does NOT invoke onCommitFailed but does warn when prior commitDelete rejects on re-schedule", async () => {
      const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
      const { result } = renderHook(useUndoDelete, { wrapper });

      const priorError = new Error("prior commit failed");
      const commitA = vi.fn().mockRejectedValue(priorError);
      const rollbackA = vi.fn();
      const failedA = vi.fn();

      act(() => {
        result.current.scheduleDelete({
          id: "dup-reject",
          label: "First delete",
          commitDelete: commitA,
          optimisticRollback: rollbackA,
          onCommitFailed: failedA,
        });
      });

      act(() => {
        result.current.scheduleDelete({
          id: "dup-reject",
          label: "Second delete",
          commitDelete: vi.fn().mockResolvedValue(undefined),
          optimisticRollback: vi.fn(),
        });
      });

      await act(async () => {
        await Promise.resolve();
      });

      expect(commitA).toHaveBeenCalledOnce();
      expect(rollbackA).not.toHaveBeenCalled();
      expect(failedA).not.toHaveBeenCalled();

      expect(warnSpy).toHaveBeenCalledWith(
        "[undo-delete] prior pending delete commit failed on re-schedule",
        expect.objectContaining({ id: "dup-reject", errName: priorError.name }),
      );

      await act(async () => {
        await vi.runAllTimersAsync();
      });

      warnSpy.mockRestore();
    });
  });
});

describe("useUndoDelete — flushPendingDeletes", () => {
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
    const { result } = renderHook(useUndoDelete, { wrapper });
    const optsA = makeOpts({ id: "flush-a" });
    const optsB = makeOpts({ id: "flush-b" });

    act(() => {
      result.current.scheduleDelete(optsA);
      result.current.scheduleDelete(optsB);
    });

    expect(result.current.pendingCount()).toBe(2);

    await act(async () => {
      await result.current.flushPendingDeletes();
    });

    expect(optsA.commitDelete).toHaveBeenCalledOnce();
    expect(optsB.commitDelete).toHaveBeenCalledOnce();
    expect(result.current.pendingCount()).toBe(0);

    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(optsA.commitDelete).toHaveBeenCalledOnce();
    expect(optsB.commitDelete).toHaveBeenCalledOnce();
  });

  it("calls optimisticRollback for any commitDelete that rejects during flush", async () => {
    const deleteError = new Error("flush commit failure");
    const { result } = renderHook(useUndoDelete, { wrapper });
    const opts = makeOpts({
      id: "flush-fail",
      commitDelete: vi.fn().mockRejectedValue(deleteError),
    });

    act(() => {
      result.current.scheduleDelete(opts);
    });

    await act(async () => {
      await result.current.flushPendingDeletes();
    });

    expect(opts.optimisticRollback).toHaveBeenCalledOnce();
    expect(opts.onCommitFailed).toHaveBeenCalledWith(deleteError);
  });

  it("concurrent flush calls do not double-commit the same item", async () => {
    const { result } = renderHook(useUndoDelete, { wrapper });
    const opts = makeOpts({ id: "flush-once" });

    act(() => {
      result.current.scheduleDelete(opts);
    });

    await act(async () => {
      await Promise.all([
        result.current.flushPendingDeletes(),
        result.current.flushPendingDeletes(),
      ]);
    });

    expect(opts.commitDelete).toHaveBeenCalledOnce();
  });
});

describe("useUndoDelete — pendingCount", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    lastUndoAction = undefined;
    toastIdCounter = 0;
    mockToastDismiss.mockReset();
  });

  afterEach(async () => {
    await act(async () => {
      await vi.runAllTimersAsync();
    });
    vi.useRealTimers();
  });

  it("returns 0 initially", () => {
    const { result } = renderHook(useUndoDelete, { wrapper });
    expect(result.current.pendingCount()).toBe(0);
  });

  it("increments when a delete is scheduled and decrements after commit", async () => {
    const { result } = renderHook(useUndoDelete, { wrapper });
    const opts = makeOpts();

    act(() => {
      result.current.scheduleDelete(opts);
    });

    expect(result.current.pendingCount()).toBe(1);

    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(result.current.pendingCount()).toBe(0);
  });

  it("decrements after undo", async () => {
    const { result } = renderHook(useUndoDelete, { wrapper });
    const opts = makeOpts();

    let handle: ReturnType<typeof result.current.scheduleDelete>;
    act(() => {
      handle = result.current.scheduleDelete(opts);
    });

    expect(result.current.pendingCount()).toBe(1);

    act(() => {
      handle?.undo();
    });

    expect(result.current.pendingCount()).toBe(0);
  });
});

describe("useUndoDelete — error when called outside Provider", () => {
  it("throws when used without UndoDeleteProvider", () => {
    // Suppress the React error boundary console.error for this test.
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => {
      renderHook(useUndoDelete);
    }).toThrow("useUndoDelete must be called inside <UndoDeleteProvider>");
    consoleSpy.mockRestore();
  });
});
