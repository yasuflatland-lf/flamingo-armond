"use client";

// cardgroupId injection happens in the parent's submit callback, not inside this component.

import { useForm } from "@tanstack/react-form";
import { useEffect, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
import { FieldError } from "@/lib/forms/field-error";
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
  onCancel?: () => void;
  /**
   * Optional callback invoked once after mount with a function the parent can
   * call to reset the form to empty values. Used by stay-on-page consecutive-add
   * flows (e.g. /cards/new) where the parent owns "submit succeeded → clear inputs"
   * and never unmounts the form between submissions.
   */
  onResetReady?: (resetFn: () => void) => void;
};

export function CardForm({
  mode,
  defaultValues,
  idPrefix = "",
  submit,
  submitLabel,
  submitting = false,
  error,
  onCancel,
  onResetReady,
}: CardFormProps) {
  const resolvedLabel = submitLabel ?? (mode === "create" ? "Add" : "Save");
  const schema = mode === "create" ? newCardSchema.omit({ cardgroupId: true }) : updateCardSchema;
  const frontSchema = schema.shape.front;
  const backSchema = schema.shape.back;

  const fieldErrors = useMemo(() => getBackendFieldErrors(error), [error]);
  const bannerError = useMemo(() => getBackendErrorBanner(error), [error]);

  const form = useForm({
    defaultValues: { front: defaultValues.front, back: defaultValues.back },
    onSubmit: async ({ value }) => {
      await submit(value).catch((err) => {
        console.error("[card-form] submit rejected", err);
        throw err;
      });
    },
  });

  // Hand the parent a stable resetter so it can clear the form after a successful
  // submit without unmounting. The dependency on `onResetReady` keeps the wiring
  // up to date if the parent ever swaps callback identity.
  useEffect(() => {
    onResetReady?.(() => form.reset({ front: "", back: "" }));
  }, [form, onResetReady]);

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
      className="space-y-3"
    >
      {bannerError ? (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {bannerError}
        </div>
      ) : null}

      <form.Field name="front" validators={{ onChange: frontSchema, onBlur: frontSchema }}>
        {(field) => {
          const inputId = `${idPrefix}${field.name}-field`;
          return (
            <div className="space-y-1">
              <Label htmlFor={inputId}>Front</Label>
              <Input
                id={inputId}
                name={field.name}
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
              />
              <FieldError zodErrors={field.state.meta.errors} backendError={fieldErrors.front} />
            </div>
          );
        }}
      </form.Field>

      <form.Field name="back" validators={{ onChange: backSchema, onBlur: backSchema }}>
        {(field) => {
          const inputId = `${idPrefix}${field.name}-field`;
          return (
            <div className="space-y-1">
              <Label htmlFor={inputId}>Back</Label>
              <Input
                id={inputId}
                name={field.name}
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
              />
              <FieldError zodErrors={field.state.meta.errors} backendError={fieldErrors.back} />
            </div>
          );
        }}
      </form.Field>

      <div className="flex items-center gap-2">
        <Button type="submit" variant="brand" disabled={submitting}>
          {submitting ? "Saving..." : resolvedLabel}
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
