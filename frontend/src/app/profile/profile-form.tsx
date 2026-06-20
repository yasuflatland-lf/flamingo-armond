"use client";

import { useForm } from "@tanstack/react-form";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { DirtyStateBridge } from "@/lib/forms/dirty-state-bridge";
import { FieldError } from "@/lib/forms/field-error";
import { submitFormHandler } from "@/lib/forms/submit-handler";
import { updateProfileSchema } from "@/schemas/profile";
import { useUpdateProfile } from "./use-update-profile";

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
  const t = useTranslations("Profile");
  const tCommon = useTranslations("Common");

  // Typed InputValidationError variant — field-level validation failure
  // surfaced by the server via the outcome union. Cleared on each new submission.
  const [validationError, setValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);

  // Mid-session auth failures or unexpected payloads. Cleared on each submission.
  const [bannerMessage, setBannerMessage] = useState<string | null>(null);

  const { submit, loading, reset } = useUpdateProfile();

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

      const outcome = await submit({
        displayName: value.displayName,
        bio: value.bio,
      });

      switch (outcome.status) {
        case "success":
          onSaved?.();
          return;
        case "validation":
          setValidationError({ field: outcome.field, message: outcome.message });
          return;
        case "unauthenticated":
          setBannerMessage(t("sessionExpired"));
          return;
        case "unexpected":
          setBannerMessage(tCommon("somethingWentWrong"));
          return;
        case "rejected":
          setBannerMessage(outcome.banner ?? tCommon("somethingWentWrong"));
          // Re-throw so TanStack Form keeps formState.isSubmitSuccessful=false;
          // submitFormHandler (the outer onSubmit) swallows the re-thrown rejection.
          throw new Error("[ProfileForm] update profile rejected");
      }
    },
  });

  // Derive per-field backend errors from the validationError state (outcome union path).
  const fieldErrors: Record<string, string | undefined> = validationError
    ? { [validationError.field]: validationError.message }
    : {};

  return (
    <form onSubmit={submitFormHandler(form)} className="space-y-4">
      {bannerMessage ? <ErrorBanner>{bannerMessage}</ErrorBanner> : null}

      <div className="mb-4 space-y-2">
        <Label>{t("email")}</Label>
        {email !== null ? <p>{email}</p> : <p className="italic">{t("noEmail")}</p>}
        <Link
          href="/profile/change-email"
          className="text-sm underline"
          onClick={(event) => {
            if (!onChangeEmail) return;
            event.preventDefault();
            onChangeEmail();
          }}
        >
          {t("changeEmail")}
        </Link>
      </div>

      <form.Field
        name="displayName"
        validators={{ onChange: displayNameSchema, onBlur: displayNameSchema }}
      >
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor={field.name}>{t("displayName")}</Label>
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
            <Label htmlFor={field.name}>{t("bio")}</Label>
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
                {t("clearBio")}
              </Button>
            ) : null}
            <FieldError zodErrors={field.state.meta.errors} backendError={fieldErrors.bio} />
          </div>
        )}
      </form.Field>

      <div className="flex items-center gap-2">
        <Button type="submit" variant="brand" disabled={loading}>
          {loading ? tCommon("saving") : tCommon("save")}
        </Button>
        {onCancel ? (
          <Button type="button" variant="outline" onClick={onCancel}>
            {tCommon("cancel")}
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
