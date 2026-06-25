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
import { masterSchema } from "@/schemas/master";
import type { AdminMasterListItem } from "./admin-master-row";

/** Values emitted by the form. Empty optional strings collapse to null. */
export type MasterFormValues = {
  name: string;
  description: string | null;
  language: string | null;
  level: string | null;
  category: string | null;
  coverImageUrl: string | null;
  source: string | null;
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
  const nameSchema = masterSchema.shape.name;
  // Cast optional-input schemas to string-input so TanStack Form's validator
  // type constraint (StandardSchemaV1<string, unknown>) is satisfied. The
  // defaultValues for these fields are always initialised to "" (never
  // undefined), so the runtime input is always a string.
  const asStringInput = (s: unknown): typeof nameSchema => s as typeof nameSchema;

  const form = useForm({
    defaultValues: {
      name: master?.name ?? "",
      description: master?.description ?? "",
      language: master?.language ?? "",
      level: master?.level ?? "",
      category: master?.category ?? "",
      coverImageUrl: master?.coverImageUrl ?? "",
      source: master?.source ?? "",
      isDefaultStarter: master?.isDefaultStarter ?? false,
      sortOrder: master?.sortOrder != null ? String(master.sortOrder) : "",
    },
    onSubmit: async ({ value }) => {
      // The per-field Zod validators only gate submission; TanStack Form does
      // not replace field values with a schema's transform output. The
      // empty-to-null collapse and the sortOrder numeric coercion below
      // intentionally re-derive what masterSchema's transforms compute, so the
      // submitted payload matches the validated shape. Keep the two in sync.
      const sortOrderRaw = value.sortOrder.trim();
      const values: MasterFormValues = {
        name: value.name.trim(),
        description: emptyToNull(value.description),
        language: emptyToNull(value.language),
        level: emptyToNull(value.level),
        category: emptyToNull(value.category),
        coverImageUrl: emptyToNull(value.coverImageUrl),
        source: emptyToNull(value.source),
        isDefaultStarter: value.isDefaultStarter,
        sortOrder: sortOrderRaw === "" ? null : Number(sortOrderRaw),
      };
      await wrapSubmit("admin-master-form", submit)(values);
    },
  });

  const nameFieldError = validationError?.field === "name" ? validationError.message : undefined;

  return (
    <form onSubmit={submitFormHandler(form)} className="space-y-4">
      {validationError && validationError.field !== "name" ? (
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
          onChange: asStringInput(masterSchema.shape.description),
          onBlur: asStringInput(masterSchema.shape.description),
          onSubmit: asStringInput(masterSchema.shape.description),
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
            <FieldError zodErrors={field.state.meta.errors} />
          </div>
        )}
      </form.Field>

      {(
        [
          ["language", "languageLabel", asStringInput(masterSchema.shape.language)],
          ["level", "levelLabel", asStringInput(masterSchema.shape.level)],
          ["category", "categoryLabel", asStringInput(masterSchema.shape.category)],
          ["coverImageUrl", "coverImageUrlLabel", asStringInput(masterSchema.shape.coverImageUrl)],
          ["source", "sourceLabel", asStringInput(masterSchema.shape.source)],
        ] as const
      ).map(([fieldName, labelKey, schema]) => (
        <form.Field
          key={fieldName}
          name={fieldName}
          validators={{ onChange: schema, onBlur: schema, onSubmit: schema }}
        >
          {(field) => (
            <div className="space-y-2">
              <Label htmlFor={field.name}>{t(labelKey)}</Label>
              <Input
                id={field.name}
                name={field.name}
                data-testid={`master-field-${fieldName}`}
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
              />
              <FieldError zodErrors={field.state.meta.errors} />
            </div>
          )}
        </form.Field>
      ))}

      <form.Field
        name="sortOrder"
        validators={{
          onChange: asStringInput(masterSchema.shape.sortOrder),
          onBlur: asStringInput(masterSchema.shape.sortOrder),
          onSubmit: asStringInput(masterSchema.shape.sortOrder),
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
