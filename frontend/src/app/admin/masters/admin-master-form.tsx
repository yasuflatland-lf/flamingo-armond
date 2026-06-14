"use client";

import { useForm } from "@tanstack/react-form";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { FieldError } from "@/lib/forms/field-error";
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
  onDelete?: (id: string) => Promise<void>;
};

function emptyToNull(s: string): string | null {
  const trimmed = s.trim();
  return trimmed.length === 0 ? null : trimmed;
}

function DirtyStateBridge({
  dirty,
  onDirtyChange,
}: {
  dirty: boolean;
  onDirtyChange?: (dirty: boolean) => void;
}) {
  useEffect(() => {
    onDirtyChange?.(dirty);
  }, [dirty, onDirtyChange]);
  return null;
}

export function AdminMasterForm({
  mode,
  master,
  submitting,
  submit,
  validationError,
  onDirtyChange,
  onDelete,
}: Props) {
  const t = useTranslations("AdminMasters");
  const tCommon = useTranslations("Common");
  const nameSchema = masterSchema.shape.name;
  const [deleting, setDeleting] = useState(false);

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
      await submit(values).catch((err) => {
        // err.message is omitted — backend messages may echo user-authored
        // input typed into this form. See
        // docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
        console.error("[admin-master-form] submit rejected", {
          name: err instanceof Error ? err.name : "unknown",
        });
        throw err; // keep formState.isSubmitSuccessful correct
      });
    },
  });

  async function handleConfirmDelete() {
    if (!master || !onDelete) return;
    setDeleting(true);
    try {
      await onDelete(master.id);
    } catch (err) {
      // Parent (client) owns the user-facing error toast; log here (redacted)
      // so the rejection is handled and never surfaces as an unhandled rejection
      // at the async onClick boundary. err.message omitted — may carry content.
      console.warn("[admin-master-form] delete rejected", {
        masterId: master.id,
        name: err instanceof Error ? err.name : "unknown",
      });
    } finally {
      setDeleting(false);
    }
  }

  const nameFieldError = validationError?.field === "name" ? validationError.message : undefined;

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        e.stopPropagation();
        form.handleSubmit().catch(() => {
          // Inner submit handler's .catch already logged; swallow here so the
          // re-thrown rejection does not surface as an unhandled browser
          // promise rejection.
        });
      }}
      className="space-y-4"
    >
      {validationError && validationError.field !== "name" ? (
        <div
          role="alert"
          data-testid="master-form-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {validationError.message}
        </div>
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

      <form.Field name="description">
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor={field.name}>{t("descriptionLabel")}</Label>
            <Textarea
              id={field.name}
              name={field.name}
              data-testid="master-field-description"
              value={field.state.value}
              onChange={(e) => field.handleChange(e.target.value)}
              placeholder={t("descriptionPlaceholder")}
            />
          </div>
        )}
      </form.Field>

      {(
        [
          ["language", "languageLabel"],
          ["level", "levelLabel"],
          ["category", "categoryLabel"],
          ["coverImageUrl", "coverImageUrlLabel"],
          ["source", "sourceLabel"],
        ] as const
      ).map(([fieldName, labelKey]) => (
        <form.Field key={fieldName} name={fieldName}>
          {(field) => (
            <div className="space-y-2">
              <Label htmlFor={field.name}>{t(labelKey)}</Label>
              <Input
                id={field.name}
                name={field.name}
                data-testid={`master-field-${fieldName}`}
                value={field.state.value}
                onChange={(e) => field.handleChange(e.target.value)}
              />
            </div>
          )}
        </form.Field>
      ))}

      <form.Field name="sortOrder">
        {(field) => (
          <div className="space-y-2">
            <Label htmlFor={field.name}>{t("sortOrderLabel")}</Label>
            <Input
              id={field.name}
              name={field.name}
              type="number"
              data-testid="master-field-sortOrder"
              value={field.state.value}
              onChange={(e) => field.handleChange(e.target.value)}
            />
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

      {mode === "edit" && master && onDelete ? (
        <div className="mt-8 space-y-2 rounded-md border border-destructive/40 p-4">
          <h2 className="text-sm font-semibold text-destructive">{t("deleteMasterHeading")}</h2>
          <p className="text-sm text-muted-foreground">{t("deleteMasterDescription")}</p>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button type="button" variant="destructive" data-testid="master-row-delete-trigger">
                {deleting ? t("deleteMasterDeleting") : t("deleteMasterButton")}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent data-testid="master-delete-dialog">
              <AlertDialogHeader>
                <AlertDialogTitle>{t("deleteMasterDialogTitle")}</AlertDialogTitle>
                <AlertDialogDescription>
                  {t("deleteMasterDialogDescription")}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel data-testid="master-delete-dialog-cancel">
                  {tCommon("cancel")}
                </AlertDialogCancel>
                <AlertDialogAction
                  data-testid="master-delete-dialog-confirm"
                  disabled={deleting}
                  onClick={handleConfirmDelete}
                >
                  {t("deleteMasterConfirmButton")}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      ) : null}

      <form.Subscribe selector={(state) => state.isDirty}>
        {(dirty) => <DirtyStateBridge dirty={dirty} onDirtyChange={onDirtyChange} />}
      </form.Subscribe>
    </form>
  );
}
