"use client";

// cardgroupId injection happens in the parent's submit callback, not inside this component.

import { useForm } from "@tanstack/react-form";
import { useTranslations } from "next-intl";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
import { FieldError } from "@/lib/forms/field-error";
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
  const resolvedLabel = submitLabel ?? (mode === "create" ? "Add" : "Save");
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
        {(field) => {
          const inputId = `${idPrefix}${field.name}-field`;
          return (
            <div className="space-y-1">
              <Label htmlFor={inputId}>{t("frontLabel")}</Label>
              <Input
                id={inputId}
                name={field.name}
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
              />
              <FieldError
                zodErrors={field.state.meta.errors}
                backendError={
                  validationError?.field === "front" ? validationError.message : fieldErrors.front
                }
              />
            </div>
          );
        }}
      </form.Field>

      <form.Field name="back" validators={{ onChange: backSchema, onBlur: backSchema }}>
        {(field) => {
          const inputId = `${idPrefix}${field.name}-field`;
          return (
            <div className="space-y-1">
              <Label htmlFor={inputId}>{t("backLabel")}</Label>
              <Input
                id={inputId}
                name={field.name}
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
              />
              <FieldError
                zodErrors={field.state.meta.errors}
                backendError={
                  validationError?.field === "back" ? validationError.message : fieldErrors.back
                }
              />
            </div>
          );
        }}
      </form.Field>

      <div className="flex items-center gap-2">
        <Button type="submit" variant="brand" disabled={submitting}>
          {submitting ? tCommon("saving") : resolvedLabel}
        </Button>
        {onCancel && (
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
        )}
      </div>
    </form>
  );
}
