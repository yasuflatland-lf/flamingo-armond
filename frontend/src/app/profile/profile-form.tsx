"use client";

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useMutation } from "@apollo/client/react";
import { useForm } from "@tanstack/react-form";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { graphql } from "@/generated";
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

function extensionString(
  extensions: Record<string, unknown> | undefined,
  key: string,
): string | undefined {
  const value = extensions?.[key];
  return typeof value === "string" ? value : undefined;
}

// Returns a map of field name -> error message for BAD_USER_INPUT errors.
function useBackendFieldErrors(err: unknown): Record<string, string> {
  if (!CombinedGraphQLErrors.is(err)) return {};
  const out: Record<string, string> = {};
  for (const ge of err.errors) {
    const code = extensionString(ge.extensions, "code");
    const field = extensionString(ge.extensions, "field");
    if (code === "BAD_USER_INPUT" && field) {
      out[field] = ge.message;
    }
  }
  return out;
}

// Returns a user-facing banner message for non-field errors.
// Priority: INTERNAL > UNAUTHENTICATED > first non-field GraphQL error > network error.
function useBackendErrorBanner(err: unknown): string | undefined {
  if (!err) return undefined;
  if (!CombinedGraphQLErrors.is(err)) {
    return "Could not reach the server. Check your connection and try again.";
  }
  let firstNonField: string | undefined;
  for (const ge of err.errors) {
    const code = extensionString(ge.extensions, "code");
    const field = extensionString(ge.extensions, "field");
    if (code === "INTERNAL") return ge.message;
    if (code === "UNAUTHENTICATED") return "Your session expired. Please sign in again.";
    if (code === "BAD_USER_INPUT" && field) continue;
    firstNonField ??= ge.message;
  }
  return firstNonField;
}

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

  const fieldErrors = useBackendFieldErrors(error);
  const bannerError = useBackendErrorBanner(error);

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
