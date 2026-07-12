"use client";

import { useForm } from "@tanstack/react-form";
import { useTranslations } from "next-intl";
import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
import { DirtyStateBridge } from "@/lib/forms/dirty-state-bridge";
import { FormField } from "@/lib/forms/form-field";
import { submitFormHandler, wrapSubmit } from "@/lib/forms/submit-handler";
import { roleSchema } from "@/schemas/role";

type RoleFormProps = {
  defaultValues: { name: string };
  submit: (values: { name: string }) => Promise<void>;
  /** Label rendered on the submit button (e.g. "Create", "Save"). */
  submitLabel: string;
  /** Parent passes Apollo mutation `loading` state. */
  submitting?: boolean;
  /** Parent passes Apollo mutation `error` for triage. */
  error?: unknown;
  /** When true, the input + submit button are disabled with no path back to enabled. */
  readOnly?: boolean;
  /** Optional cancel action rendered next to the submit button. */
  onCancel?: () => void;
  onDirtyChange?: (dirty: boolean) => void;
  /**
   * Fires on each field value change (the create sheet uses it to clear a stale
   * validation banner as the user edits — the explicit replacement for the
   * former `<div onInput>` wrapper).
   */
  onEdit?: () => void;
};

export function RoleForm({
  defaultValues,
  submit,
  submitLabel,
  submitting = false,
  error,
  readOnly = false,
  onCancel,
  onDirtyChange,
  onEdit,
}: RoleFormProps) {
  const t = useTranslations("Admin");
  const tCommon = useTranslations("Common");
  const nameSchema = roleSchema.shape.name;

  const fieldErrors = useMemo(() => getBackendFieldErrors(error), [error]);
  const bannerError = useMemo(() => getBackendErrorBanner(error), [error]);

  const form = useForm({
    defaultValues: {
      name: defaultValues.name,
    },
    listeners: {
      onChange: () => onEdit?.(),
    },
    onSubmit: async ({ value }) => {
      await wrapSubmit("role-form", submit)(value);
    },
  });

  return (
    <form onSubmit={submitFormHandler(form)} className="space-y-4">
      {bannerError ? <ErrorBanner>{bannerError}</ErrorBanner> : null}

      <form.Field name="name" validators={{ onChange: nameSchema, onBlur: nameSchema }}>
        {(field) => (
          <FormField
            field={field}
            label={t("roleNameLabel")}
            backendError={fieldErrors.name}
            disabled={readOnly}
            placeholder={t("roleNamePlaceholder")}
          />
        )}
      </form.Field>

      <div className="flex items-center gap-2">
        <Button type="submit" variant="brand" disabled={submitting || readOnly}>
          {submitting ? tCommon("saving") : submitLabel}
        </Button>
        {onCancel ? (
          <Button type="button" variant="outline" onClick={onCancel}>
            {tCommon("cancel")}
          </Button>
        ) : null}
      </div>
      <form.Subscribe selector={(state) => state.isDirty}>
        {(dirty) => <DirtyStateBridge dirty={dirty} onDirtyChange={onDirtyChange} />}
      </form.Subscribe>
    </form>
  );
}
