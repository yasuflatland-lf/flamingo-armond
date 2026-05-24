"use client";

import { useMutation } from "@apollo/client/react";
import { useCallback, useEffect, useState } from "react";
import { CreateCardMutation } from "@/app/cardgroups/queries";
import { CardForm } from "@/components/cardgroups/card-form";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";

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
  const [open, setOpen] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [validationError, setValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);

  const [createCard, { loading: creating, error: createError, reset: resetCreateCard }] =
    useMutation(CreateCardMutation);

  const openSheet = useCallback(() => {
    resetCreateCard();
    setValidationError(null);
    setDirty(false);
    setOpen(true);
  }, [resetCreateCard]);

  useEffect(() => {
    function handleAddCardEvent(event: Event) {
      if (!(event instanceof CustomEvent)) return;
      const detail = event.detail as { cardgroupId?: unknown } | null;
      if (detail?.cardgroupId !== cardgroupId) return;

      event.preventDefault();
      openSheet();
    }

    window.addEventListener("flamingo:add-card", handleAddCardEvent);
    return () => window.removeEventListener("flamingo:add-card", handleAddCardEvent);
  }, [cardgroupId, openSheet]);

  async function handleCreate(values: { front: string; back: string }) {
    resetCreateCard();
    setValidationError(null);
    const result = await createCard({
      variables: { input: { cardgroupId, front: values.front, back: values.back } },
    }).catch((err) => {
      // err.message is omitted — backend messages may echo user-authored content.
      console.error("[LearnAddCardSheet] create rejection", {
        name: err instanceof Error ? err.name : "unknown",
        cardgroupId,
      });
      return null;
    });
    if (!result) return;

    const payload = result.data?.createCard;
    if (payload?.__typename === "CreateCardSuccess") {
      setDirty(false);
      setValidationError(null);
      setOpen(false);
    } else if (payload?.__typename === "CardDuplicateFrontError") {
      setValidationError({ field: "front", message: payload.message });
    } else {
      const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
      console.warn("[LearnAddCardSheet] unexpected createCard payload", {
        typename: unknownPayload?.__typename ?? null,
        cardgroupId,
      });
      setValidationError({ field: "front", message: "Add failed. Please try again." });
    }
  }

  return (
    <FormSheet
      title="Add card"
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
