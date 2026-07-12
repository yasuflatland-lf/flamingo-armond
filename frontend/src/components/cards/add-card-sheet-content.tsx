"use client";

import { Check } from "lucide-react";
import { useTranslations } from "next-intl";
import { CardForm } from "@/components/cardgroups/card-form";
import { useFormSheetClose } from "@/components/ui/form-sheet";

/**
 * Shared "add card" drawer body used by the cards screen
 * (`/cardgroups/[id]/cards`) and the learn screen's in-context add sheet.
 *
 * `idPrefix` namespaces the form field ids per call site. When `addedCount`
 * is provided and greater than zero, a success row surfaces the running
 * continuous-add tally; the learn screen omits `addedCount` so the row never
 * renders there.
 */
export function AddCardSheetContent({
  idPrefix,
  submit,
  submitting,
  error,
  validationError,
  onDirty,
  addedCount,
}: {
  idPrefix: string;
  submit: (values: { front: string; back: string }) => Promise<void>;
  submitting: boolean;
  error: unknown;
  validationError: { field: string; message: string } | null;
  onDirty: () => void;
  addedCount?: number;
}) {
  const close = useFormSheetClose();
  const t = useTranslations("Cards");

  return (
    <div onInput={onDirty} className="space-y-3">
      {addedCount !== undefined && addedCount > 0 ? (
        <p
          className="flex items-center gap-1.5 text-sm text-success"
          data-testid="add-card-added-count"
        >
          <Check aria-hidden="true" className="h-4 w-4" />
          {t("addedCount", { count: addedCount })}
        </p>
      ) : null}
      <CardForm
        mode="create"
        idPrefix={idPrefix}
        defaultValues={{ front: "", back: "" }}
        submit={submit}
        submitting={submitting}
        error={error}
        validationError={validationError}
        onCancel={close}
      />
    </div>
  );
}
