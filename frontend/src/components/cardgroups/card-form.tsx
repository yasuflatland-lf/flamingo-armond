"use client";

// cardgroupId injection happens in the parent's submit callback, not inside this component.

import { useForm } from "@tanstack/react-form";
import { useTranslations } from "next-intl";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
import { FormField } from "@/lib/forms/form-field";
import { submitFormHandler, wrapSubmit } from "@/lib/forms/submit-handler";
import { newCardSchema, updateCardSchema } from "@/schemas/card";

type Mode = "create" | "edit";

export type CardFormProps = {
  mode: Mode;
  defaultValues: { front: string; back: string };
  /** Optional prefix for input element IDs; useful when multiple CardForms are on the page. */
  idPrefix?: string;
  submit: (values: { front: string; back: string }) => Promise<void>;
  submitLabel?: string;
  submitting?: boolean;
  error?: unknown;
  /**
   * Typed InputValidationError variant surfaced by outcome-union mutations.
   * When present, takes precedence over `error` for the matched field so the
   * inline field error shows the server message instead of the
   * substring-matched `BAD_USER_INPUT` text.
   */
  validationError?: { field: string; message: string } | null;
  onCancel?: () => void;
};

export function CardForm({
  mode,
  defaultValues,
  idPrefix = "",
  submit,
  submitLabel,
  submitting = false,
  error,
  validationError,
  onCancel,
}: CardFormProps) {
  const t = useTranslations("Cards");
  const tCommon = useTranslations("Common");
  const resolvedLabel = submitLabel ?? (mode === "create" ? t("add") : tCommon("save"));
  const schema = mode === "create" ? newCardSchema.omit({ cardgroupId: true }) : updateCardSchema;
  const frontSchema = schema.shape.front;
  const backSchema = schema.shape.back;

  const fieldErrors = useMemo(() => getBackendFieldErrors(error), [error]);
  const bannerError = useMemo(() => getBackendErrorBanner(error), [error]);

  const form = useForm({
    defaultValues: { front: defaultValues.front, back: defaultValues.back },
    onSubmit: async ({ value }) => {
      await wrapSubmit("card-form", submit)(value);
    },
  });

  return (
    <form onSubmit={submitFormHandler(form)} className="space-y-3">
      {bannerError ? <ErrorBanner>{bannerError}</ErrorBanner> : null}

      <form.Field name="front" validators={{ onChange: frontSchema, onBlur: frontSchema }}>
        {(field) => (
          <FormField
            field={field}
            label={t("frontLabel")}
            idOverride={`${idPrefix}${field.name}-field`}
            className="space-y-1"
            backendError={
              validationError?.field === "front" ? validationError.message : fieldErrors.front
            }
          />
        )}
      </form.Field>

      <form.Field name="back" validators={{ onChange: backSchema, onBlur: backSchema }}>
        {(field) => (
          <FormField
            field={field}
            label={t("backLabel")}
            idOverride={`${idPrefix}${field.name}-field`}
            className="space-y-1"
            backendError={
              validationError?.field === "back" ? validationError.message : fieldErrors.back
            }
          />
        )}
      </form.Field>

      <div className="flex items-center gap-2">
        <Button type="submit" variant="brand" disabled={submitting}>
          {submitting ? tCommon("saving") : resolvedLabel}
        </Button>
        {onCancel && (
          <Button type="button" variant="outline" onClick={onCancel}>
            {tCommon("cancel")}
          </Button>
        )}
      </div>
    </form>
  );
}
