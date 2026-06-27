"use client";

import { useForm } from "@tanstack/react-form";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { DirtyStateBridge } from "@/lib/forms/dirty-state-bridge";
import { FormField } from "@/lib/forms/form-field";
import { submitFormHandler, wrapSubmit } from "@/lib/forms/submit-handler";
import { masterDescriptionSchema, masterNameSchema, masterSortOrderSchema } from "@/schemas/master";
import type { AdminMasterListItem } from "./admin-master-row";

/** Values emitted by the form. Empty optional strings collapse to null. */
export type MasterFormValues = {
  name: string;
  description: string | null;
  isDefaultStarter: boolean;
  sortOrder: number | null;
};

type Props = {
  mode: "create" | "edit";
  master?: AdminMasterListItem;
  submitting: boolean;
  submit: (values: MasterFormValues) => Promise<void>;
  validationError?: { field: string; message: string } | null;
  onDirtyChange?: (dirty: boolean) => void;
};

function emptyToNull(s: string): string | null {
  const trimmed = s.trim();
  return trimmed.length === 0 ? null : trimmed;
}

export function AdminMasterForm({
  mode,
  master,
  submitting,
  submit,
  validationError,
  onDirtyChange,
}: Props) {
  const t = useTranslations("AdminMasters");
  const tCommon = useTranslations("Common");
  const nameSchema = masterNameSchema;
  const descriptionSchema = masterDescriptionSchema;
  const sortOrderSchema = masterSortOrderSchema;

  const form = useForm({
    defaultValues: {
      name: master?.name ?? "",
      description: master?.description ?? "",
      isDefaultStarter: master?.isDefaultStarter ?? false,
      sortOrder: master?.sortOrder != null ? String(master.sortOrder) : "",
    },
    onSubmit: async ({ value }) => {
      const sortOrderRaw = value.sortOrder.trim();
      const parsedSortOrder = Number(sortOrderRaw);
      const values: MasterFormValues = {
        name: value.name.trim(),
        description: emptyToNull(value.description),
        isDefaultStarter: value.isDefaultStarter,
        // The sortOrder field validator blocks submit on a non-integer; guard the
        // conversion too so NaN / Infinity can never reach the mutation.
        sortOrder:
          sortOrderRaw !== "" && Number.isInteger(parsedSortOrder) ? parsedSortOrder : null,
      };
      await wrapSubmit("admin-master-form", submit)(values);
    },
  });

  const nameFieldError = validationError?.field === "name" ? validationError.message : undefined;
  const descriptionFieldError =
    validationError?.field === "description" ? validationError.message : undefined;

  return (
    <form onSubmit={submitFormHandler(form)} className="space-y-4">
      {validationError &&
      validationError.field !== "name" &&
      validationError.field !== "description" ? (
        <ErrorBanner data-testid="master-form-error">{validationError.message}</ErrorBanner>
      ) : null}

      <form.Field
        name="name"
        validators={{ onChange: nameSchema, onBlur: nameSchema, onSubmit: nameSchema }}
      >
        {(field) => (
          <FormField
            field={field}
            kind="text"
            label={t("nameLabel")}
            testId="master-field-name"
            backendError={nameFieldError}
            placeholder={t("namePlaceholder")}
          />
        )}
      </form.Field>

      <form.Field
        name="description"
        validators={{
          onChange: descriptionSchema,
          onBlur: descriptionSchema,
          onSubmit: descriptionSchema,
        }}
      >
        {(field) => (
          <FormField
            field={field}
            kind="textarea"
            label={t("descriptionLabel")}
            testId="master-field-description"
            backendError={descriptionFieldError}
            placeholder={t("descriptionPlaceholder")}
          />
        )}
      </form.Field>

      <form.Field
        name="sortOrder"
        validators={{
          onChange: sortOrderSchema,
          onBlur: sortOrderSchema,
          onSubmit: sortOrderSchema,
        }}
      >
        {(field) => (
          <FormField
            field={field}
            kind="number"
            label={t("sortOrderLabel")}
            testId="master-field-sortOrder"
          />
        )}
      </form.Field>

      <form.Field name="isDefaultStarter">
        {(field) => (
          <FormField
            field={field}
            kind="checkbox"
            label={t("isDefaultStarterLabel")}
            testId="master-field-isDefaultStarter"
          />
        )}
      </form.Field>

      <div className="flex items-center gap-2 border-t pt-4">
        <Button
          type="submit"
          variant="brand"
          disabled={submitting}
          data-testid="master-form-submit"
        >
          {submitting ? tCommon("saving") : mode === "create" ? t("createMaster") : tCommon("save")}
        </Button>
      </div>

      <form.Subscribe selector={(state) => state.isDirty}>
        {(dirty) => <DirtyStateBridge dirty={dirty} onDirtyChange={onDirtyChange} />}
      </form.Subscribe>
    </form>
  );
}
