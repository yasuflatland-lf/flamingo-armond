"use client";

import { useForm } from "@tanstack/react-form";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { DirtyStateBridge } from "@/lib/forms/dirty-state-bridge";
import { FieldError } from "@/lib/forms/field-error";
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
          <div className="space-y-2">
            <Label htmlFor={field.name}>{t("nameLabel")}</Label>
            <Input
              id={field.name}
              name={field.name}
              data-testid="master-field-name"
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.target.value)}
              placeholder={t("namePlaceholder")}
            />
            <FieldError zodErrors={field.state.meta.errors} backendError={nameFieldError} />
          </div>
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
          <div className="space-y-2">
            <Label htmlFor={field.name}>{t("descriptionLabel")}</Label>
            <Textarea
              id={field.name}
              name={field.name}
              data-testid="master-field-description"
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.target.value)}
              placeholder={t("descriptionPlaceholder")}
            />
            <FieldError zodErrors={field.state.meta.errors} backendError={descriptionFieldError} />
          </div>
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
          <div className="space-y-2">
            <Label htmlFor={field.name}>{t("sortOrderLabel")}</Label>
            <Input
              id={field.name}
              name={field.name}
              type="number"
              data-testid="master-field-sortOrder"
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.target.value)}
            />
            <FieldError zodErrors={field.state.meta.errors} />
          </div>
        )}
      </form.Field>

      <form.Field name="isDefaultStarter">
        {(field) => (
          <div className="flex items-center gap-2">
            <input
              id={field.name}
              name={field.name}
              type="checkbox"
              data-testid="master-field-isDefaultStarter"
              checked={field.state.value}
              onChange={(e) => field.handleChange(e.target.checked)}
              className="h-4 w-4 rounded border-input"
            />
            <Label htmlFor={field.name}>{t("isDefaultStarterLabel")}</Label>
          </div>
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
