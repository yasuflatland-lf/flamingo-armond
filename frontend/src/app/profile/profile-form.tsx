"use client";

import { useMutation } from "@apollo/client/react";
import { useForm } from "@tanstack/react-form";
import { useRouter } from "next/navigation";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { graphql } from "@/generated";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
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

function hasMessage(value: unknown): value is { message: string } {
  return typeof (value as { message?: unknown })?.message === "string";
}

type FieldErrorProps = { zodErrors: unknown[]; backendError?: string };

function FieldError({ zodErrors, backendError }: FieldErrorProps) {
  const msg = zodErrors.find(hasMessage)?.message ?? backendError;
  if (!msg) return null;
  return <p className="text-sm text-destructive">{msg}</p>;
}

type Props = { initial: { displayName: string; bio: string } };

export function ProfileForm({ initial }: Props) {
  const router = useRouter();
  const [updateProfile, { loading, error }] = useMutation(UpdateProfileMutation, {
    onCompleted: () => router.refresh(),
  });

  const fieldErrors = useMemo(() => getBackendFieldErrors(error), [error]);
  const bannerError = useMemo(() => getBackendErrorBanner(error), [error]);

  const displayNameSchema = updateProfileSchema.shape.displayName;
  const bioSchema = updateProfileSchema.shape.bio;

  const form = useForm({
    defaultValues: {
      displayName: initial.displayName,
      bio: initial.bio as string | undefined,
    },
    onSubmit: async ({ value }) => {
      // bio: "" clears, undefined leaves unchanged. Errors surface via the mutation's error state;
      // the catch prevents unhandled rejections without swallowing diagnostics.
      await updateProfile({
        variables: {
          input: {
            displayName: value.displayName,
            bio: value.bio,
          },
        },
      }).catch((err) => {
        console.error("[ProfileForm] mutation rejection", err);
      });
    },
  });

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        e.stopPropagation();
        void form.handleSubmit();
      }}
      className="space-y-4"
    >
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

      <Button type="submit" disabled={loading}>
        {loading ? "Saving..." : "Save"}
      </Button>
    </form>
  );
}
