"use client";

import { useForm } from "@tanstack/react-form";
import type React from "react";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
import { FieldError } from "@/lib/forms/field-error";
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
  /** Parent passes Apollo mutation `error` for triage. */
  error?: unknown;
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
  secondarySlot,
}: CardgroupFormProps) {
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
      await submit(value).catch((err) => {
        console.error("[cardgroup-form] submit rejected", err);
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

      <form.Field name="name" validators={{ onChange: nameSchema, onBlur: nameSchema }}>
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor={field.name}>Name</Label>
            <Input
              id={field.name}
              name={field.name}
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.target.value)}
            />
            <FieldError zodErrors={field.state.meta.errors} backendError={fieldErrors.name} />
          </div>
        )}
      </form.Field>

      <div className="flex items-center gap-2">
        <Button type="submit" disabled={submitting}>
          {submitting ? "Saving..." : resolvedLabel}
        </Button>
        {secondarySlot}
      </div>
    </form>
  );
}
