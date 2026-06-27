"use client";

import { useCallback, useState } from "react";
import type { CreateCardgroupOutcome } from "./use-create-cardgroup";
import { useCreateCardgroup } from "./use-create-cardgroup";

/**
 * Form state machine for create-cardgroup outcome routing and error display.
 * Wraps the IO-only {@link useCreateCardgroup} hook (which owns the mutation,
 * the Connection cache write, and the outcome narrowing) and adds the form-side
 * error state — validation, auth, limit, and unexpected — that both create
 * clients previously hand-rolled identically. Mirrors the thin-outcome-hook
 * shape of `useRoleMutations` / `useSheetForm`: own the field-level error,
 * delegate caller-specific side effects (drawer close, navigation) to the
 * consumer.
 *
 * `submit(values)` clears all error state, calls the create mutation, routes the
 * returned outcome into the matching error state, and returns the raw outcome so
 * the caller can handle `success` (navigation / drawer close) without re-parsing
 * the union.
 *
 * Callers translate the structured error state to display copy:
 * - `validationError` → passed directly to `CardgroupForm`'s `validationError` prop
 * - `authError` → resolved to banner copy for `AuthErrorBanner`
 * - `limitError` → translated via i18n `t("limitReached", { limit, current })`
 * - `unexpectedError` → translated via i18n `tCommon("somethingWentWrong")`;
 *   the discriminant (`"unexpected"` vs `"rejected"`) lets each client decide
 *   whether a transport rejection surfaces a banner (the modal drawer does; the
 *   full-page route stays silent).
 *
 * The underlying mutation carries no `optimisticResponse`: typed errors
 * (`InputValidationError`, `CardgroupLimitReachedError`, `FORBIDDEN`) can fail it
 * and Apollo does not reliably roll back optimistic writes for typed GraphQL
 * errors — see .claude/rules/pagination.md.
 */
export function useCreateCardgroupForm() {
  const { create: createCardgroup, loading } = useCreateCardgroup();

  const [validationError, setValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);
  const [authError, setAuthError] = useState<"unauthenticated" | "forbidden" | null>(null);
  const [limitError, setLimitError] = useState<{ limit: number; current: number } | null>(null);
  const [unexpectedError, setUnexpectedError] = useState<"unexpected" | "rejected" | null>(null);

  const reset = useCallback(() => {
    setValidationError(null);
    setAuthError(null);
    setLimitError(null);
    setUnexpectedError(null);
  }, []);

  const submit = useCallback(
    async (values: { name: string }): Promise<CreateCardgroupOutcome> => {
      reset();
      const outcome = await createCardgroup(values.name);

      switch (outcome.status) {
        case "validation":
          setValidationError({ field: outcome.field, message: outcome.message });
          break;
        case "auth":
          setAuthError(outcome.kind);
          break;
        case "limit":
          setLimitError({ limit: outcome.limit, current: outcome.current });
          break;
        case "unexpected":
          setUnexpectedError("unexpected");
          break;
        case "rejected":
          setUnexpectedError("rejected");
          break;
        case "success":
          // Success: no error state to set; the caller navigates / closes.
          break;
      }

      return outcome;
    },
    [createCardgroup, reset],
  );

  return {
    submit,
    loading,
    validationError,
    authError,
    limitError,
    unexpectedError,
    reset,
  };
}
