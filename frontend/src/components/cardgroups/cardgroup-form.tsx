"use client";

import { useForm } from "@tanstack/react-form";
import { useTranslations } from "next-intl";
import type React from "react";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
import { FieldError } from "@/lib/forms/field-error";
import { submitFormHandler, wrapSubmit } from "@/lib/forms/submit-handler";
import { newCardgroupSchema, updateCardgroupSchema } from "@/schemas/cardgroup";

type Mode = "create" | "edit";

type CardgroupFormProps = {
  mode: Mode;
  defaultValues: { name: string };
  submit: (values: { name: string }) => Promise<void>;
  /** Defaults to "Create" in create mode, "Save" in edit mode. */
  submitLabel?: string;
  /** Parent passes Apollo mutation `loading` state. */
  submitting?: boolean;
  /** Parent passes Apollo mutation `error` for triage. Used by non-promoted callers (updateCardgroup). */
  error?: unknown;
  /**
   * Typed InputValidationError variant surfaced by outcome-union mutations.
   * When present, takes precedence over `error` for the `name` field so the
   * inline field error shows the server message instead of the
   * substring-matched `BAD_USER_INPUT` text.
   */
  validationError?: { field: string; message: string } | null;
  /** Extra controls rendered next to the submit button (e.g. Delete button on Edit page). */
  secondarySlot?: React.ReactNode;
};

export function CardgroupForm({
  mode,
  defaultValues,
  submit,
  submitLabel,
  submitting = false,
  error,
  validationError,
  secondarySlot,
}: CardgroupFormProps) {
  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");
  const resolvedLabel = submitLabel ?? (mode === "create" ? "Create" : "Save");

  const schema = mode === "create" ? newCardgroupSchema : updateCardgroupSchema;
  const nameSchema = schema.shape.name;

  const fieldErrors = useMemo(() => getBackendFieldErrors(error), [error]);
  const bannerError = useMemo(() => getBackendErrorBanner(error), [error]);

  const form = useForm({
    defaultValues: {
      name: defaultValues.name,
    },
    onSubmit: async ({ value }) => {
      await wrapSubmit("cardgroup-form", submit)(value);
    },
  });

  return (
    <form onSubmit={submitFormHandler(form)} className="space-y-4">
      {bannerError ? <ErrorBanner>{bannerError}</ErrorBanner> : null}

      <form.Field name="name" validators={{ onChange: nameSchema, onBlur: nameSchema }}>
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor={field.name}>{t("nameLabel")}</Label>
            <Input
              id={field.name}
              name={field.name}
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.target.value)}
            />
            <FieldError
              zodErrors={field.state.meta.errors}
              backendError={
                validationError?.field === "name" ? validationError.message : fieldErrors.name
              }
            />
          </div>
        )}
      </form.Field>

      <div className="flex items-center gap-2">
        <Button type="submit" variant="brand" disabled={submitting}>
          {submitting ? tCommon("saving") : resolvedLabel}
        </Button>
        {secondarySlot}
      </div>
    </form>
  );
}
