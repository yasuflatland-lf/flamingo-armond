"use client";

import { useCallback, useState } from "react";

/** Field-level validation error surfaced inline under a form control. */
export type ValidationError = { field: string; message: string };

function isValidationOutcome(outcome: {
  status: string;
}): outcome is { status: "validation"; field: string; message: string } {
  return outcome.status === "validation" && "field" in outcome && "message" in outcome;
}

/**
 * Owns one sheet/form's field-level `validationError` and routes a mutation
 * outcome union. `run(submit)` clears the error, awaits `submit()`, then:
 * `success` → calls `onSuccess`; `validation` → stores the field error; any
 * other status → leaves the error cleared and returns the (concrete) outcome so
 * the caller can handle its own extra variants (`unexpected` / `rejected` /
 * `auth` / `unauthenticated` / …).
 */
export function useSheetForm(onSuccess: () => void) {
  const [validationError, setValidationError] = useState<ValidationError | null>(null);
  const clearValidationError = useCallback(() => setValidationError(null), []);

  const run = useCallback(
    async <T extends { status: string }>(submit: () => Promise<T>): Promise<T> => {
      setValidationError(null);
      const outcome = await submit();
      if (outcome.status === "success") {
        onSuccess();
      } else if (isValidationOutcome(outcome)) {
        setValidationError({ field: outcome.field, message: outcome.message });
      }
      return outcome;
    },
    [onSuccess],
  );

  return { validationError, setValidationError, clearValidationError, run };
}

type CardValues = { front: string; back: string };

type CardOutcome =
  | { status: "success" }
  | { status: "validation"; field: string; message: string }
  | { status: "unexpected" }
  | { status: "rejected" };

export type UseCardSheetFormInput = {
  createCard: (values: CardValues) => Promise<CardOutcome>;
  updateCard: (id: string, values: CardValues) => Promise<CardOutcome>;
  /** Resets the create mutation's Apollo error/loading state. */
  resetCreateCard: () => void;
  /** Already-translated fallback shown when a create fails unexpectedly. */
  addFailedMessage: string;
  /** Already-translated fallback shown when an update fails unexpectedly. */
  saveFailedMessage: string;
};

/**
 * The add/edit-card sheet state machine shared, byte-for-byte, by the cardgroup
 * and master-deck card screens. Owns the add-sheet open/dirty flags, the editing
 * row id, both field-level validation errors, and the create/update outcome
 * routing (incl. the close-on-success and `unexpected` → inline-`front`-error
 * fallbacks). The caller supplies the mutation runners (from
 * `useCardMutations` / `useMasterCardMutations`) and the localized fallbacks.
 */
export function useCardSheetForm({
  createCard,
  updateCard,
  resetCreateCard,
  addFailedMessage,
  saveFailedMessage,
}: UseCardSheetFormInput) {
  const [addOpen, setAddOpen] = useState(false);
  const [addDirty, setAddDirty] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  const createForm = useSheetForm(
    useCallback(() => {
      setAddDirty(false);
      setAddOpen(false);
    }, []),
  );
  const updateForm = useSheetForm(useCallback(() => setEditingId(null), []));

  const {
    clearValidationError: clearCreateError,
    run: runCreate,
    setValidationError: setCreateError,
  } = createForm;
  const {
    clearValidationError: clearUpdateError,
    run: runUpdate,
    setValidationError: setUpdateError,
  } = updateForm;

  const openAddSheet = useCallback(() => {
    resetCreateCard();
    clearCreateError();
    setAddDirty(false);
    setAddOpen(true);
  }, [resetCreateCard, clearCreateError]);

  const markAddDirty = useCallback(() => setAddDirty(true), []);

  const onAddOpenChange = useCallback(
    (nextOpen: boolean) => {
      setAddOpen(nextOpen);
      if (!nextOpen) {
        resetCreateCard();
        setAddDirty(false);
        clearCreateError();
      }
    },
    [resetCreateCard, clearCreateError],
  );

  const onEditOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) {
        clearUpdateError();
        setEditingId(null);
      }
    },
    [clearUpdateError],
  );

  const beginEdit = useCallback(
    (id: string) => {
      clearUpdateError();
      setEditingId(id);
    },
    [clearUpdateError],
  );

  const handleCreate = useCallback(
    async (values: CardValues) => {
      resetCreateCard();
      const outcome = await runCreate(() => createCard(values));
      if (outcome.status === "unexpected") {
        setCreateError({ field: "front", message: addFailedMessage });
      }
    },
    [resetCreateCard, runCreate, setCreateError, createCard, addFailedMessage],
  );

  const handleUpdate = useCallback(
    async (id: string, values: CardValues) => {
      const outcome = await runUpdate(() => updateCard(id, values));
      if (outcome.status === "unexpected") {
        setUpdateError({ field: "front", message: saveFailedMessage });
      }
    },
    [runUpdate, setUpdateError, updateCard, saveFailedMessage],
  );

  return {
    addOpen,
    addDirty,
    markAddDirty,
    editingId,
    createValidationError: createForm.validationError,
    rowValidationError: updateForm.validationError,
    openAddSheet,
    onAddOpenChange,
    onEditOpenChange,
    beginEdit,
    handleCreate,
    handleUpdate,
  };
}
