"use client";

import { CardForm } from "@/components/cardgroups/card-form";

export type EditCardRowProps = {
  card: { id: string; front: string; back: string };
  submit: (values: { front: string; back: string }) => Promise<void>;
  submitting: boolean;
  error: unknown;
  validationError: { field: string; message: string } | null;
  onCancel: () => void;
};

export function EditCardRow({
  card,
  submit,
  submitting,
  error,
  validationError,
  onCancel,
}: EditCardRowProps) {
  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: stopPropagation prevents bubbling to the parent's edit-toggle handler; this <li> is not interactive while CardForm is shown.
    <li className="rounded-md border border-border p-4" onClick={(e) => e.stopPropagation()}>
      <CardForm
        mode="edit"
        idPrefix={`edit-${card.id}-`}
        defaultValues={{ front: card.front, back: card.back }}
        submit={submit}
        submitLabel="Save"
        submitting={submitting}
        error={error}
        validationError={validationError}
        onCancel={onCancel}
      />
    </li>
  );
}
