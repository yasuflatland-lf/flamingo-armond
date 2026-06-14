"use client";

import { useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { graphql } from "@/generated";
import type { UpdateProfileInput } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";

const UpdateProfileMutation = graphql(`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input) {
      __typename
      ... on UpdateProfileSuccess {
        user {
          id
          displayName
          bio
          avatarUrl
        }
      }
      ... on InputValidationError {
        field
        message
      }
    }
  }
`);

/**
 * Discriminated outcome of an update-profile attempt. Both `/profile`
 * (`ProfileForm`) and `/onboarding` (`OnboardingForm`) consume this hook and
 * branch on `status`:
 *
 * - `success`        — the caller runs its own success behaviour (profile calls
 *   `onSaved()`; onboarding navigates to `/onboarding/start`).
 * - `validation`     — a typed `InputValidationError`; the caller surfaces the
 *   `message` under the named `field`.
 * - `unauthenticated`— the session expired mid-submit; the caller shows its own
 *   sign-in banner. The submit completes normally (no re-throw).
 * - `unexpected`     — a null payload, partial-response null bubble, or a future
 *   union variant the client was not regenerated against; the caller shows a
 *   generic banner. The submit completes normally (no re-throw).
 * - `rejected`       — a non-auth transport / `INTERNAL` error. `banner` carries
 *   the derived backend message (`null` when none could be extracted, so the
 *   caller falls back to its own generic copy). Forms backed by TanStack Form
 *   re-throw on this outcome to keep `formState.isSubmitSuccessful` false.
 *
 * Mirrors the established feature-hook shape of `useImportMaster` /
 * `useCreateCardgroup`: the hook owns the mutation document and the typed-outcome
 * classification; the caller owns presentation (i18n copy, navigation, re-throw).
 */
export type UpdateProfileOutcome =
  | { status: "success" }
  | { status: "validation"; field: string; message: string }
  | { status: "unauthenticated" }
  | { status: "unexpected" }
  | { status: "rejected"; banner: string | null };

/**
 * Shared `updateProfile` mutation + typed-outcome translation. Declares the
 * `UpdateProfile` document once (previously duplicated byte-for-byte between
 * `profile-form.tsx` and `onboarding-form.tsx`) and classifies every response
 * into an `UpdateProfileOutcome`.
 *
 * `reset` is the underlying Apollo mutation reset, surfaced for `ProfileForm`'s
 * `onRegisterReset` flow; onboarding does not use it.
 */
export function useUpdateProfile() {
  const [updateProfile, { loading, reset }] = useMutation(UpdateProfileMutation);

  const submit = useCallback(
    async (input: UpdateProfileInput): Promise<UpdateProfileOutcome> => {
      try {
        const result = await updateProfile({ variables: { input } });
        const payload = result.data?.updateProfile;
        // Capture typename before narrowing so the unknown-variant branch can log
        // it (TypeScript narrows to `never` after the known cases).
        const typename = payload?.__typename ?? null;
        if (payload?.__typename === "InputValidationError") {
          return { status: "validation", field: payload.field, message: payload.message };
        }
        if (payload?.__typename === "UpdateProfileSuccess") {
          return { status: "success" };
        }
        // Null payload, partial-response null bubble, or a future union variant
        // the client was not regenerated against.
        console.warn("[useUpdateProfile] unexpected updateProfile payload", { typename });
        return { status: "unexpected" };
      } catch (err) {
        const codes = liftGraphQLCodes(err);
        if (codes.includes("UNAUTHENTICATED")) return { status: "unauthenticated" };
        // err.message is omitted — backend messages may echo user input.
        // codes is safe to log (fixed enum of GraphQL extension codes).
        console.warn("[useUpdateProfile] updateProfile rejected", {
          name: err instanceof Error ? err.name : "unknown",
          codes,
        });
        return { status: "rejected", banner: getBackendErrorBanner(err) ?? null };
      }
    },
    [updateProfile],
  );

  return { submit, loading, reset };
}
