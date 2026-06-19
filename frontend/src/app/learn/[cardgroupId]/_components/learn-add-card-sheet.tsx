"use client";

import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";
import { cardsDefaultVars } from "@/app/cardgroups/[id]/cards/queries";
import { useCardMutations } from "@/app/cardgroups/[id]/cards/use-card-mutations";
import { CardForm } from "@/components/cardgroups/card-form";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import { type AddCardDetail, FLAMINGO_EVENT } from "@/lib/events/flamingo-events";

function AddCardSheetContent({
  submit,
  submitting,
  error,
  validationError,
  onDirty,
}: {
  submit: (values: { front: string; back: string }) => Promise<void>;
  submitting: boolean;
  error: unknown;
  validationError: { field: string; message: string } | null;
  onDirty: () => void;
}) {
  const close = useFormSheetClose();

  return (
    <div onInput={onDirty}>
      <CardForm
        mode="create"
        idPrefix="learn-add-card-"
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

/**
 * In-context "add card" drawer for the Learn screen. Mounted independently of
 * the swipe queue so the '+' works even when the queue is empty (AllCaughtUp).
 *
 * Opened by the `flamingo:add-card` event the LogoDrawer '+' button dispatches;
 * the listener calls preventDefault so the dispatcher skips its fallback
 * navigation. A created card is saved and the drawer closes — it surfaces in a
 * later session via FSRS scheduling, leaving the current swipe queue untouched.
 */
export function LearnAddCardSheet({ cardgroupId }: { cardgroupId: string }) {
  const t = useTranslations("Cards");
  const [open, setOpen] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [validationError, setValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);

  // The Learn screen has no cards-connection subscription of its own, so the
  // shared hook's create path seeds the cardgroup's default-vars connection
  // (search: null) — the same create slice the cards screen uses.
  const queryVariables = useMemo(() => cardsDefaultVars(cardgroupId), [cardgroupId]);
  const { createCard, creating, createError, resetCreateCard } = useCardMutations({
    cardgroupId,
    queryVariables,
  });

  const openSheet = useCallback(() => {
    resetCreateCard();
    setValidationError(null);
    setDirty(false);
    setOpen(true);
  }, [resetCreateCard]);

  useEffect(() => {
    function handleAddCardEvent(event: CustomEvent<AddCardDetail>) {
      if (event.detail?.cardgroupId !== cardgroupId) return;

      event.preventDefault();
      openSheet();
    }

    window.addEventListener(FLAMINGO_EVENT.addCard, handleAddCardEvent);
    return () => window.removeEventListener(FLAMINGO_EVENT.addCard, handleAddCardEvent);
  }, [cardgroupId, openSheet]);

  async function handleCreate(values: { front: string; back: string }) {
    resetCreateCard();
    setValidationError(null);
    const outcome = await createCard(values);
    if (outcome.status === "success") {
      setDirty(false);
      setValidationError(null);
      setOpen(false);
    } else if (outcome.status === "validation") {
      setValidationError({ field: outcome.field, message: outcome.message });
    } else if (outcome.status === "unexpected") {
      setValidationError({ field: "front", message: "Add failed. Please try again." });
    }
    // outcome.status === "rejected": the hook already logged the rejection;
    // leave the sheet open so the user can retry.
  }

  return (
    <FormSheet
      title={t("addCardSheetTitle")}
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen);
        if (!nextOpen) {
          resetCreateCard();
          setDirty(false);
          setValidationError(null);
        }
      }}
      submitting={creating}
      dirty={dirty}
      confirmOnDismiss
    >
      <AddCardSheetContent
        submit={handleCreate}
        submitting={creating}
        error={createError}
        validationError={validationError}
        onDirty={() => setDirty(true)}
      />
    </FormSheet>
  );
}
