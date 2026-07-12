"use client";

import { useForm } from "@tanstack/react-form";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { Label } from "@/components/ui/label";
import { DirtyStateBridge } from "@/lib/forms/dirty-state-bridge";
import { FormField } from "@/lib/forms/form-field";
import { submitFormHandler } from "@/lib/forms/submit-handler";
import { updateProfileSchema } from "@/schemas/profile";
import { useUpdateProfileSubmit } from "./use-update-profile-submit";

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

  const onSuccess = useCallback(() => onSaved?.(), [onSaved]);

  const { submit, bannerMessage, fieldErrors, loading, reset } = useUpdateProfileSubmit({
    onSuccess,
    sessionExpiredMessage: t("sessionExpired"),
    rejectionLabel: "[ProfileForm] update profile rejected",
  });

  useEffect(() => {
    onRegisterReset?.(reset);
  }, [onRegisterReset, reset]);

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
      await submit({
        displayName: value.displayName,
        bio: value.bio,
      });
    },
  });

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
          <FormField
            field={field}
            label={t("displayName")}
            backendError={fieldErrors.displayName}
          />
        )}
      </form.Field>

      <form.Field name="bio" validators={{ onChange: bioSchema, onBlur: bioSchema }}>
        {(field) => (
          <FormField
            field={field}
            kind="textarea"
            label={t("bio")}
            backendError={fieldErrors.bio}
            trailing={
              field.state.value !== "" ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => field.handleChange("")}
                >
                  {t("clearBio")}
                </Button>
              ) : null
            }
          />
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
