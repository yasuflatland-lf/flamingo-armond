"use client";

import { useTranslations } from "next-intl";
import { useCallback, useState } from "react";
import type { UpdateProfileInput } from "@/generated/graphql";
import { useUpdateProfile } from "./use-update-profile";

/** Field-scoped backend validation error surfaced by the `InputValidationError` outcome variant. */
type FieldValidationError = { field: string; message: string };

type UseUpdateProfileSubmitParams = {
  /**
   * Runs on the `success` outcome. `ProfileForm` calls `onSaved?.()`; `OnboardingForm`
   * sets its `navigating` flag and pushes `/onboarding/start`. Wrap in `useCallback` at the
   * caller so the returned `submit` identity stays stable across renders.
   */
  onSuccess: () => void;
  /**
   * Localized banner for the `unauthenticated` (session-expired) outcome. The copy differs
   * between the Profile and Onboarding namespaces, so the caller owns it (mirrors the
   * caller-owns-i18n contract documented on `useUpdateProfile`).
   */
  sessionExpiredMessage: string;
  /**
   * Error message thrown on the `rejected` outcome so TanStack Form keeps
   * `formState.isSubmitSuccessful` false. `submitFormHandler` (the outer `onSubmit`) swallows
   * the re-thrown rejection.
   */
  rejectionLabel: string;
};

/**
 * Presentation-side wrapper over {@link useUpdateProfile}. Owns the `validationError` /
 * `bannerMessage` state and the outcome switch (including the `rejected` re-throw) that
 * `ProfileForm` and `OnboardingForm` previously duplicated byte-for-byte.
 *
 * The generic "something went wrong" copy is resolved internally from the shared `Common`
 * namespace; the namespace-specific `sessionExpiredMessage` and the form-specific
 * `rejectionLabel` are caller-owned params. Returns `{ submit, validationError, bannerMessage,
 * fieldErrors, loading, reset }`:
 *
 * - `submit(input)` clears the error state, runs the outcome switch, calls `onSuccess` on
 *   success, and re-throws (`rejectionLabel`) on the `rejected` outcome.
 * - `reset()` clears the local error state and resets the underlying Apollo mutation — used by
 *   `ProfileForm`'s `onRegisterReset` flow; onboarding does not use it.
 */
export function useUpdateProfileSubmit({
  onSuccess,
  sessionExpiredMessage,
  rejectionLabel,
}: UseUpdateProfileSubmitParams) {
  const tCommon = useTranslations("Common");
  const { submit: submitProfile, loading, reset: resetMutation } = useUpdateProfile();

  // Typed InputValidationError variant — field-level validation failure surfaced by the server
  // via the outcome union. Cleared on each new submission.
  const [validationError, setValidationError] = useState<FieldValidationError | null>(null);

  // Mid-session auth failures or unexpected payloads. Cleared on each submission.
  const [bannerMessage, setBannerMessage] = useState<string | null>(null);

  const submit = useCallback(
    async (input: UpdateProfileInput): Promise<void> => {
      setValidationError(null);
      setBannerMessage(null);

      const outcome = await submitProfile(input);

      switch (outcome.status) {
        case "success":
          onSuccess();
          return;
        case "validation":
          setValidationError({ field: outcome.field, message: outcome.message });
          return;
        case "unauthenticated":
          setBannerMessage(sessionExpiredMessage);
          return;
        case "unexpected":
          setBannerMessage(tCommon("somethingWentWrong"));
          return;
        case "rejected":
          setBannerMessage(outcome.banner ?? tCommon("somethingWentWrong"));
          // Re-throw so TanStack Form keeps formState.isSubmitSuccessful=false;
          // submitFormHandler (the outer onSubmit) swallows the re-thrown rejection.
          throw new Error(rejectionLabel);
      }
    },
    [submitProfile, onSuccess, sessionExpiredMessage, rejectionLabel, tCommon],
  );

  const reset = useCallback(() => {
    setValidationError(null);
    setBannerMessage(null);
    resetMutation();
  }, [resetMutation]);

  // Derive per-field backend errors from the validationError state (outcome union path).
  const fieldErrors: Record<string, string | undefined> = validationError
    ? { [validationError.field]: validationError.message }
    : {};

  return { submit, validationError, bannerMessage, fieldErrors, loading, reset };
}
