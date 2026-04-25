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

// Returns a map of field name -> error message for BAD_USER_INPUT errors.
function useBackendFieldErrors(err: unknown): Record<string, string> {
  if (!CombinedGraphQLErrors.is(err)) return {};
  const out: Record<string, string> = {};
  for (const ge of err.errors) {
    const code = ge.extensions?.code as string | undefined;
    const field = ge.extensions?.field as string | undefined;
    if (code === "BAD_USER_INPUT" && field) {
      out[field] = ge.message;
    }
  }
  return out;
}

// Returns the first INTERNAL error message, or undefined.
function useBackendInternalError(err: unknown): string | undefined {
  if (!CombinedGraphQLErrors.is(err)) return undefined;
  for (const ge of err.errors) {
    const code = ge.extensions?.code as string | undefined;
    if (code === "INTERNAL") return ge.message;
  }
  return undefined;
}

function FieldError({ zodErrors, backendError }: { zodErrors: unknown[]; backendError?: string }) {
  const z = zodErrors.find(
    (e): e is { message: string } => typeof (e as { message?: unknown })?.message === "string",
  );
  const msg = z?.message ?? backendError;
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
  const internalError = useBackendInternalError(error);

  const displayNameSchema = updateProfileSchema.shape.displayName;
  const bioSchema = updateProfileSchema.shape.bio;

  const form = useForm({
    defaultValues: {
      displayName: initial.displayName,
      bio: initial.bio as string | undefined,
    },
    onSubmit: async ({ value }) => {
      const bioInput = value.bio === "" ? "" : value.bio === undefined ? undefined : value.bio;
      await updateProfile({
        variables: {
          input: {
            displayName: value.displayName,
            bio: bioInput,
          },
        },
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
      {internalError ? (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {internalError}
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
