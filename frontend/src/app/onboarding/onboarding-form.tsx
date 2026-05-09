"use client";

import { useMutation } from "@apollo/client/react";
import { useForm } from "@tanstack/react-form";
import { useRouter } from "next/navigation";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { graphql } from "@/generated";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
import { FieldError } from "@/lib/forms/field-error";
import { updateProfileSchema } from "@/schemas/profile";

const UpdateProfileMutation = graphql(`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input) {
      user {
        id
        displayName
        bio
        avatarUrl
      }
    }
  }
`);

export function OnboardingForm() {
  const router = useRouter();
  const [updateProfile, { loading, error }] = useMutation(UpdateProfileMutation, {
    onCompleted: () => router.push("/cardgroups/new?welcome=1"),
  });

  const fieldErrors = useMemo(() => getBackendFieldErrors(error), [error]);
  const bannerError = useMemo(() => getBackendErrorBanner(error), [error]);

  const displayNameSchema = updateProfileSchema.shape.displayName;

  const form = useForm({
    defaultValues: {
      displayName: "",
    },
    onSubmit: async ({ value }) => {
      // Errors surface via the mutation's error state; the catch prevents
      // unhandled rejections without swallowing diagnostics.
      await updateProfile({
        variables: {
          input: {
            displayName: value.displayName,
          },
        },
      }).catch((err) => {
        console.error("[OnboardingForm] mutation rejection", err);
        throw err; // keep formState.isSubmitSuccessful correct
      });
    },
  });

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
      <h1 className="mb-6 text-2xl font-semibold">Welcome to flamingo</h1>

      {bannerError ? (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {bannerError}
        </div>
      ) : null}

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
            <p className="text-sm text-muted-foreground">
              1–50 characters; visible to other users.
            </p>
            <FieldError
              zodErrors={field.state.meta.errors}
              backendError={fieldErrors.displayName}
            />
          </div>
        )}
      </form.Field>

      <Button type="submit" disabled={loading}>
        {loading ? "Saving..." : "Continue"}
      </Button>

      {/* Hidden sentinel used by tests to observe formState.isSubmitSuccessful */}
      <form.Subscribe selector={(state) => state.isSubmitSuccessful}>
        {(isSubmitSuccessful) => (
          <span data-testid="is-submit-successful" data-value={String(isSubmitSuccessful)} hidden />
        )}
      </form.Subscribe>
    </form>
  );
}
