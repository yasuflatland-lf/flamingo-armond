// @vitest-environment happy-dom
import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useCardSheetForm, useSheetForm } from "./use-sheet-form";

describe("useSheetForm", () => {
  it("calls onSuccess and leaves validationError null on a success outcome", async () => {
    const onSuccess = vi.fn();
    const { result } = renderHook(() => useSheetForm(onSuccess));
    await act(async () => {
      await result.current.run(() => Promise.resolve({ status: "success" as const }));
    });
    expect(onSuccess).toHaveBeenCalledOnce();
    expect(result.current.validationError).toBeNull();
  });

  it("stores the field error on a validation outcome and does not call onSuccess", async () => {
    const onSuccess = vi.fn();
    const { result } = renderHook(() => useSheetForm(onSuccess));
    await act(async () => {
      await result.current.run(() =>
        Promise.resolve({ status: "validation" as const, field: "front", message: "too long" }),
      );
    });
    expect(onSuccess).not.toHaveBeenCalled();
    expect(result.current.validationError).toEqual({ field: "front", message: "too long" });
  });

  it("returns a non-routed outcome untouched (caller handles extra variants)", async () => {
    const { result } = renderHook(() => useSheetForm(vi.fn()));
    let returned: { status: string } | undefined;
    await act(async () => {
      returned = await result.current.run(() => Promise.resolve({ status: "unexpected" as const }));
    });
    expect(returned).toEqual({ status: "unexpected" });
    expect(result.current.validationError).toBeNull();
  });

  it("clears a prior validationError at the start of the next run", async () => {
    const { result } = renderHook(() => useSheetForm(vi.fn()));
    act(() => result.current.setValidationError({ field: "front", message: "stale" }));
    expect(result.current.validationError).not.toBeNull();
    await act(async () => {
      await result.current.run(() => Promise.resolve({ status: "rejected" as const }));
    });
    expect(result.current.validationError).toBeNull();
  });
});

type CardOutcome =
  | { status: "success" }
  | { status: "validation"; field: string; message: string }
  | { status: "unexpected" }
  | { status: "rejected" };

function setup(overrides?: { createOutcome?: CardOutcome; updateOutcome?: CardOutcome }) {
  const createCard = vi.fn(
    async () => overrides?.createOutcome ?? ({ status: "success" } as const),
  );
  const updateCard = vi.fn(
    async () => overrides?.updateOutcome ?? ({ status: "success" } as const),
  );
  const resetCreateCard = vi.fn();
  const hook = renderHook(() =>
    useCardSheetForm({
      createCard,
      updateCard,
      resetCreateCard,
      addFailedMessage: "Could not add",
      saveFailedMessage: "Could not save",
    }),
  );
  return { ...hook, createCard, updateCard, resetCreateCard };
}

describe("useCardSheetForm", () => {
  it("starts closed and clean", () => {
    const { result } = setup();
    expect(result.current.addOpen).toBe(false);
    expect(result.current.addDirty).toBe(false);
    expect(result.current.editingId).toBeNull();
    expect(result.current.createValidationError).toBeNull();
    expect(result.current.rowValidationError).toBeNull();
  });

  it("openAddSheet opens the sheet and resets create state", () => {
    const { result, resetCreateCard } = setup();
    act(() => result.current.markAddDirty());
    act(() => result.current.openAddSheet());
    expect(result.current.addOpen).toBe(true);
    expect(result.current.addDirty).toBe(false);
    expect(resetCreateCard).toHaveBeenCalled();
  });

  it("markAddDirty flips the add-sheet dirty flag", () => {
    const { result } = setup();
    act(() => result.current.markAddDirty());
    expect(result.current.addDirty).toBe(true);
  });

  it("handleCreate success keeps the sheet open and clears dirty", async () => {
    const { result } = setup({ createOutcome: { status: "success" } });
    act(() => result.current.openAddSheet());
    await act(async () => {
      await result.current.handleCreate({ front: "f", back: "b" });
    });
    expect(result.current.addOpen).toBe(true);
    expect(result.current.addDirty).toBe(false);
    expect(result.current.createValidationError).toBeNull();
  });

  it("handleCreate validation surfaces the server field error", async () => {
    const { result } = setup({
      createOutcome: { status: "validation", field: "front", message: "duplicate" },
    });
    await act(async () => {
      await result.current.handleCreate({ front: "f", back: "b" });
    });
    expect(result.current.createValidationError).toEqual({ field: "front", message: "duplicate" });
  });

  it("handleCreate unexpected falls back to the localized front error", async () => {
    const { result } = setup({ createOutcome: { status: "unexpected" } });
    await act(async () => {
      await result.current.handleCreate({ front: "f", back: "b" });
    });
    expect(result.current.createValidationError).toEqual({
      field: "front",
      message: "Could not add",
    });
  });

  it("handleCreate rejected sets no inline error (banner owns it)", async () => {
    const { result } = setup({ createOutcome: { status: "rejected" } });
    await act(async () => {
      await result.current.handleCreate({ front: "f", back: "b" });
    });
    expect(result.current.createValidationError).toBeNull();
  });

  it("handleUpdate rejected sets no inline row error and keeps the sheet open", async () => {
    const { result } = setup({ updateOutcome: { status: "rejected" } });
    act(() => result.current.beginEdit("card-1"));
    await act(async () => {
      await result.current.handleUpdate("card-1", { front: "f", back: "b" });
    });
    expect(result.current.rowValidationError).toBeNull();
    // rejected does not clear editingId — only a success outcome closes the edit sheet.
    expect(result.current.editingId).toBe("card-1");
  });

  it("beginEdit selects the row and handleUpdate success clears it", async () => {
    const { result } = setup({ updateOutcome: { status: "success" } });
    act(() => result.current.beginEdit("card-1"));
    expect(result.current.editingId).toBe("card-1");
    await act(async () => {
      await result.current.handleUpdate("card-1", { front: "f", back: "b" });
    });
    expect(result.current.editingId).toBeNull();
  });

  it("handleUpdate validation surfaces the inline row error", async () => {
    const { result } = setup({
      updateOutcome: { status: "validation", field: "back", message: "required" },
    });
    act(() => result.current.beginEdit("card-1"));
    await act(async () => {
      await result.current.handleUpdate("card-1", { front: "f", back: "b" });
    });
    expect(result.current.rowValidationError).toEqual({ field: "back", message: "required" });
    expect(result.current.editingId).toBe("card-1");
  });

  it("handleUpdate unexpected falls back to the localized front error", async () => {
    const { result } = setup({ updateOutcome: { status: "unexpected" } });
    act(() => result.current.beginEdit("card-1"));
    await act(async () => {
      await result.current.handleUpdate("card-1", { front: "f", back: "b" });
    });
    expect(result.current.rowValidationError).toEqual({
      field: "front",
      message: "Could not save",
    });
  });

  it("onAddOpenChange(false) clears create state", async () => {
    const { result, resetCreateCard } = setup({
      createOutcome: { status: "validation", field: "front", message: "dup" },
    });
    act(() => result.current.openAddSheet());
    await act(async () => {
      await result.current.handleCreate({ front: "f", back: "b" });
    });
    expect(result.current.createValidationError).not.toBeNull();
    resetCreateCard.mockClear();
    act(() => result.current.onAddOpenChange(false));
    expect(result.current.addOpen).toBe(false);
    expect(result.current.addDirty).toBe(false);
    expect(result.current.createValidationError).toBeNull();
    expect(resetCreateCard).toHaveBeenCalled();
  });

  it("onEditOpenChange(false) clears the editing row", () => {
    const { result } = setup();
    act(() => result.current.beginEdit("card-1"));
    act(() => result.current.onEditOpenChange(false));
    expect(result.current.editingId).toBeNull();
    expect(result.current.rowValidationError).toBeNull();
  });
});

function setupContinuous(createResult: { status: string } = { status: "success" }) {
  return renderHook(() =>
    useCardSheetForm({
      createCard: vi.fn().mockResolvedValue(createResult),
      updateCard: vi.fn().mockResolvedValue({ status: "success" }),
      resetCreateCard: vi.fn(),
      addFailedMessage: "add failed",
      saveFailedMessage: "save failed",
    }),
  );
}

describe("useCardSheetForm continuous add", () => {
  it("keeps the sheet open and increments addedCount + createNonce on success", async () => {
    const { result } = setupContinuous({ status: "success" });
    act(() => result.current.openAddSheet());
    expect(result.current.addOpen).toBe(true);
    expect(result.current.addedCount).toBe(0);
    const nonce0 = result.current.createNonce;

    await act(async () => {
      await result.current.handleCreate({ front: "a", back: "b" });
    });
    expect(result.current.addOpen).toBe(true);
    expect(result.current.addedCount).toBe(1);
    expect(result.current.createNonce).not.toBe(nonce0);

    await act(async () => {
      await result.current.handleCreate({ front: "c", back: "d" });
    });
    expect(result.current.addedCount).toBe(2);
  });

  it("resets addedCount when the sheet is dismissed", async () => {
    const { result } = setupContinuous({ status: "success" });
    act(() => result.current.openAddSheet());
    await act(async () => {
      await result.current.handleCreate({ front: "a", back: "b" });
    });
    expect(result.current.addedCount).toBe(1);
    act(() => result.current.onAddOpenChange(false));
    expect(result.current.addedCount).toBe(0);
    expect(result.current.addOpen).toBe(false);
  });
});
