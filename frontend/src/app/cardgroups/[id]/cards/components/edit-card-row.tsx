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
  );
}
