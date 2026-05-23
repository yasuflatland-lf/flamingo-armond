"use client";

import { useMutation } from "@apollo/client/react";
import { useForm } from "@tanstack/react-form";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { graphql } from "@/generated";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { FieldError } from "@/lib/forms/field-error";
import { updateProfileSchema } from "@/schemas/profile";

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

type Props = {
  /** The user's email address. Required — callers must pass the value or explicit null; never collapse to "". */
  email: string | null;
  initial: { displayName: string; bio: string };
  onChangeEmail?: () => void;
  onCancel?: () => void;
  onDirtyChange?: (dirty: boolean) => void;
  onRegisterReset?: (reset: () => void) => void;
  onSaved?: () => void;
  onSubmittingChange?: (submitting: boolean) => void;
};

function DirtyStateBridge({
  dirty,
  onDirtyChange,
}: {
  dirty: boolean;
  onDirtyChange?: (dirty: boolean) => void;
}) {
  useEffect(() => {
    onDirtyChange?.(dirty);
  }, [dirty, onDirtyChange]);

  return null;
}

export function ProfileForm({
  email,
  initial,
  onCancel,
  onChangeEmail,
  onDirtyChange,
  onRegisterReset,
  onSaved,
  onSubmittingChange,
}: Props) {
  // Typed InputValidationError variant — field-level validation failure
  // surfaced by the server via the outcome union. Cleared on each new submission.
  const [validationError, setValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);

  // Mid-session auth failures or unexpected payloads. Cleared on each submission.
  const [bannerMessage, setBannerMessage] = useState<string | null>(null);

  const [updateProfile, { loading, reset }] = useMutation(UpdateProfileMutation);

  const resetLocalState = useCallback(() => {
    setValidationError(null);
    setBannerMessage(null);
    reset();
  }, [reset]);

  useEffect(() => {
    onRegisterReset?.(resetLocalState);
  }, [onRegisterReset, resetLocalState]);

  useEffect(() => {
    onSubmittingChange?.(loading);
  }, [loading, onSubmittingChange]);

  const displayNameSchema = updateProfileSchema.shape.displayName;
  const bioSchema = updateProfileSchema.shape.bio;

  const form = useForm({
    defaultValues: {
      displayName: initial.displayName,
      bio: initial.bio as string | undefined,
    },
    onSubmit: async ({ value }) => {
      setValidationError(null);
      setBannerMessage(null);

      const result = await updateProfile({
        variables: {
          input: {
            displayName: value.displayName,
            bio: value.bio,
          },
        },
      }).catch((err) => {
        const codes = liftGraphQLCodes(err);
        if (codes.includes("UNAUTHENTICATED")) {
          setBannerMessage("Your session expired. Please sign in again.");
          return null;
        }
        const banner = getBackendErrorBanner(err) ?? "Something went wrong. Please try again.";
        setBannerMessage(banner);
        console.error("[ProfileForm] mutation rejection", err);
        throw err; // keep formState.isSubmitSuccessful correct
      });

      if (!result) return;

      const payload = result.data?.updateProfile;

      if (payload?.__typename === "InputValidationError") {
        setValidationError({ field: payload.field, message: payload.message });
        return;
      }

      if (payload?.__typename === "UpdateProfileSuccess") {
        onSaved?.();
        return;
      }

      // Unknown variant: null payload or a future union variant the client was not
      // regenerated against.
      const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
      console.warn("[ProfileForm] unexpected updateProfile payload", {
        typename: unknownPayload?.__typename ?? null,
      });
      setBannerMessage("Something went wrong. Please try again.");
    },
  });

  // Derive per-field backend errors from the validationError state (outcome union path).
  const fieldErrors: Record<string, string | undefined> = validationError
    ? { [validationError.field]: validationError.message }
    : {};

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        e.stopPropagation();
        form.handleSubmit().catch(() => {
          // The inner submit handler's .catch already logged; swallow here so the
          // re-thrown rejection (which keeps formState.isSubmitSuccessful=false correct)
          // does not surface as an unhandled browser promise rejection.
        });
      }}
      className="space-y-4"
    >
      {bannerMessage ? (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {bannerMessage}
        </div>
      ) : null}

      <div className="mb-4 space-y-2">
        <Label>Email</Label>
        {email !== null ? <p>{email}</p> : <p className="italic">No email on this account</p>}
        <Link
          href="/profile/change-email"
          className="text-sm underline"
          onClick={(event) => {
            if (!onChangeEmail) return;
            event.preventDefault();
            onChangeEmail();
          }}
        >
          Change email
        </Link>
      </div>

      <form.Field
        name="displayName"
        validators={{ onChange: displayNameSchema, onBlur: displayNameSchema }}
      >
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor={field.name}>Display name</Label>
            <Input
              id={field.name}
              name={field.name}
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.target.value)}
            />
            <FieldError
              zodErrors={field.state.meta.errors}
              backendError={fieldErrors.displayName}
            />
          </div>
        )}
      </form.Field>

      <form.Field name="bio" validators={{ onChange: bioSchema, onBlur: bioSchema }}>
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor={field.name}>Bio</Label>
            <Textarea
              id={field.name}
              name={field.name}
              value={field.state.value ?? ""}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.target.value)}
            />
            {field.state.value !== "" ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => field.handleChange("")}
              >
                Clear bio
              </Button>
            ) : null}
            <FieldError zodErrors={field.state.meta.errors} backendError={fieldErrors.bio} />
          </div>
        )}
      </form.Field>

      <div className="flex items-center gap-2">
        <Button type="submit" variant="brand" disabled={loading}>
          {loading ? "Saving..." : "Save"}
        </Button>
        {onCancel ? (
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
        ) : null}
      </div>

      {/* Hidden sentinel used by tests to observe formState.isSubmitSuccessful */}
      <form.Subscribe selector={(state) => state.isSubmitSuccessful}>
        {(isSubmitSuccessful) => (
          <span data-testid="is-submit-successful" data-value={String(isSubmitSuccessful)} hidden />
        )}
      </form.Subscribe>
      <form.Subscribe selector={(state) => state.isDirty}>
        {(dirty) => <DirtyStateBridge dirty={dirty} onDirtyChange={onDirtyChange} />}
      </form.Subscribe>
    </form>
  );
}
