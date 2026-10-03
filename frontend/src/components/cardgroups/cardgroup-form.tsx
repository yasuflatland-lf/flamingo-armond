"use client";

import { useForm } from "@tanstack/react-form";
import { useTranslations } from "next-intl";
import type React from "react";
import { Button } from "@/components/ui/button";
import { DirtyStateBridge } from "@/lib/forms/dirty-state-bridge";
import { FormField } from "@/lib/forms/form-field";
import { submitFormHandler, wrapSubmit } from "@/lib/forms/submit-handler";
import { cardgroupSchema } from "@/schemas/cardgroup";

type Mode = "create" | "edit";

type CardgroupFormProps = {
  mode: Mode;
  defaultValues: { name: string };
  submit: (values: { name: string }) => Promise<void>;
  /** Parent passes Apollo mutation `loading` state. */
  submitting?: boolean;
  /** Typed InputValidationError from outcome-union mutations; only field === "name" is rendered, other fields are ignored. */
  validationError?: { field: string; message: string } | null;
  /** Extra controls rendered next to the submit button (e.g. Delete button on Edit page). */
  secondarySlot?: React.ReactNode;
  /** Notifies the parent sheet of the form's TanStack `isDirty` state (drives the discard guard). */
  onDirtyChange?: (dirty: boolean) => void;
};

export function CardgroupForm({
  mode,
  defaultValues,
  submit,
  submitting = false,
  validationError,
  secondarySlot,
  onDirtyChange,
}: CardgroupFormProps) {
  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");
  const resolvedLabel = mode === "create" ? tCommon("create") : tCommon("save");

  const nameSchema = cardgroupSchema.shape.name;

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
      <form.Field name="name" validators={{ onChange: nameSchema, onBlur: nameSchema }}>
        {(field) => (
          <FormField
            field={field}
            label={t("nameLabel")}
            backendError={validationError?.field === "name" ? validationError.message : undefined}
          />
        )}
      </form.Field>

      <div className="flex items-center gap-2">
        <Button
          type="submit"
          variant="brand"
          disabled={submitting}
          data-testid="cardgroup-form-submit"
        >
          {submitting ? tCommon("saving") : resolvedLabel}
        </Button>
        {secondarySlot}
      </div>
      <form.Subscribe selector={(state) => state.isDirty}>
        {(dirty) => <DirtyStateBridge dirty={dirty} onDirtyChange={onDirtyChange} />}
      </form.Subscribe>
    </form>
  );
}
