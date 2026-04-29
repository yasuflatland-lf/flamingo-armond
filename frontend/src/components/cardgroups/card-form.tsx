"use client";

// Shared inline form for both "add card" and "edit card" modes.
// Create mode uses newCardSchema (front+back+cardgroupId); edit mode uses updateCardSchema (front+back).
// cardgroupId is supplied as a prop and injected at submit time to keep the form fields consistent.

import { useForm } from "@tanstack/react-form";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
import { cn } from "@/lib/utils";
import { newCardSchema, updateCardSchema } from "@/schemas/card";

function hasMessage(value: unknown): value is { message: string } {
  return typeof (value as { message?: unknown })?.message === "string";
}

type FieldErrorProps = { zodErrors: unknown[]; backendError?: string };

function FieldError({ zodErrors, backendError }: FieldErrorProps) {
  const msg = zodErrors.find(hasMessage)?.message ?? backendError;
  if (!msg) return null;
  return <p className="text-sm text-destructive">{msg}</p>;
}

type Mode = "create" | "edit";

export type CardFormProps = {
  mode: Mode;
  defaultValues: { front: string; back: string };
  /** Required in create mode; ignored in edit mode (cardgroupId not part of update). */
  cardgroupId?: string;
  /** Optional prefix for input element IDs; useful when multiple CardForms are on the page. */
  idPrefix?: string;
  submit: (values: { front: string; back: string }) => Promise<void>;
  submitLabel?: string;
  submitting?: boolean;
  error?: unknown;
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
  onCancel,
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
      className={cn("space-y-3")}
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
        <Button type="submit" disabled={submitting}>
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
