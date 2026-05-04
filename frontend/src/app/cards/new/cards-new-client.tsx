"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { sanitizeReturnTo } from "@/app/cardgroups/new/page";
import { CreateCardMutation, UpdateCardMutation } from "@/app/cardgroups/queries";
import { SetLastViewedCardgroupMutation } from "@/app/learn/queries";
import { CardForm } from "@/components/cardgroups/card-form";
import { CardgroupChip } from "@/components/cardgroups/cardgroup-chip";
import CardgroupPickerSheet from "@/components/cardgroups/cardgroup-picker-sheet";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
import { tryGetDuplicateCardInfo } from "@/lib/apollo/graphql-errors";

type Cardgroup = {
  id: string;
  name: string;
};

type Props = {
  /** Server-resolved cardgroup id (may be null when no cardgroup is pre-selected). */
  initialCardgroupId: string | null;
  /** When true the picker opens immediately on mount (no pre-selected cardgroup). */
  forcePickerOpen: boolean;
  /** Cardgroups owned by the current user, seeded server-side. */
  myCardgroups: Cardgroup[];
};

/**
 * Holds the duplicate-card payload returned by the backend plus the user's
 * attempted submission, so the overwrite-confirm dialog can render a
 * side-by-side comparison without re-reading from form state.
 */
type DuplicateState = {
  existingCardId: string;
  existingBack: string;
  attemptedFront: string;
  attemptedBack: string;
} | null;

function DuplicateOverwriteDialog({
  duplicate,
  onConfirm,
  onCancel,
  loading,
  error,
}: {
  duplicate: NonNullable<DuplicateState>;
  onConfirm: () => void;
  onCancel: () => void;
  loading: boolean;
  error: string | null;
}) {
  return (
    <AlertDialog
      open
      onOpenChange={(open) => {
        // Radix calls onOpenChange(false) for Escape, outside-click and (by
        // default) for Action/Cancel button clicks. We suppress the Action
        // auto-close via event.preventDefault() in onConfirm so the dialog
        // can stay open if the overwrite mutation fails; everything else
        // (Escape, outside-click, the Cancel button) routes to onCancel.
        if (!open) onCancel();
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Card already exists</AlertDialogTitle>
          <AlertDialogDescription>
            A card with the front &ldquo;{duplicate.attemptedFront}&rdquo; already exists in this
            cardgroup. Overwrite its back with your new content? Learning history is preserved.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div className="space-y-1">
            <div className="text-xs font-medium text-muted-foreground">Existing back</div>
            <pre className="whitespace-pre-wrap break-words rounded-md border bg-muted/40 p-2 text-sm">
              {duplicate.existingBack}
            </pre>
          </div>
          <div className="space-y-1">
            <div className="text-xs font-medium text-muted-foreground">New back</div>
            <pre className="whitespace-pre-wrap break-words rounded-md border bg-muted/40 p-2 text-sm">
              {duplicate.attemptedBack}
            </pre>
          </div>
        </div>

        {error ? (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
            {error}
          </div>
        ) : null}

        <AlertDialogFooter>
          <AlertDialogCancel disabled={loading}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={(e) => {
              // Suppress Radix's default close-on-action behaviour. Closing is
              // driven by the parent clearing `duplicate` after a successful
              // overwrite; on failure the dialog must stay mounted so the user
              // can retry or cancel.
              e.preventDefault();
              onConfirm();
            }}
            disabled={loading}
          >
            {loading ? "Overwriting…" : "Overwrite"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export default function CardsNewClient({
  initialCardgroupId,
  forcePickerOpen,
  myCardgroups,
}: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [pickerOpen, setPickerOpen] = useState(forcePickerOpen);

  // URL is the single source of truth; fall back to server-resolved id.
  const urlCardgroupId = searchParams.get("cardgroup");
  const currentId = urlCardgroupId ?? initialCardgroupId;

  // Sanitize the ?return= param so an open-redirect cannot be introduced via
  // user-supplied input. sanitizeReturnTo rejects external URLs (protocol-
  // relative or scheme-bearing) and returns null for anything that is not a
  // safe internal path. The type is string | null (required, not optional) so
  // downstream branches must acknowledge the absence explicitly.
  const returnTo: string | null = sanitizeReturnTo(searchParams.get("return") ?? undefined);

  const currentName =
    currentId != null ? (myCardgroups.find((cg) => cg.id === currentId)?.name ?? null) : null;

  const [createCard, { loading: creating, error: createError }] = useMutation(CreateCardMutation);
  // Do not carry an `optimisticResponse` for either mutation. setLastViewed can
  // fail typed (BAD_USER_INPUT when the cardgroup was deleted between page render
  // and submit) and Apollo v3.x does not roll back optimistic writes on typed
  // errors — see .claude/rules/pagination.md.
  const [setLastViewed] = useMutation(SetLastViewedCardgroupMutation);
  // Same Apollo v3.x rollback caveat as createCard above (UNAUTHENTICATED on
  // session expiry, BAD_USER_INPUT from validators).
  const [updateCard, { loading: overwriting }] = useMutation(UpdateCardMutation);

  const [duplicate, setDuplicate] = useState<DuplicateState>(null);
  // Inline error rendered inside DuplicateOverwriteDialog when updateCard fails.
  // Sibling state (rather than reusing createError) so the dialog stays open
  // and the message survives even after the create-mutation hook resets.
  const [overwriteError, setOverwriteError] = useState<string | null>(null);

  // Shared post-success tail for both the create and overwrite paths:
  // fire-and-forget the last-viewed hint (must not block the UI; setLastViewed
  // can fail typed when the cardgroup was deleted between page render and
  // submit), then navigate away. When a ?return= param was supplied (and
  // passed the open-redirect guard above), push to that path; otherwise push
  // to the cardgroup's cards list. See .claude/rules/pagination.md on dropping
  // optimisticResponse for typed-fail mutations.
  function markCreationSucceeded() {
    if (!currentId) return;
    void setLastViewed({ variables: { cardgroupId: currentId } }).catch((err) => {
      console.warn("[cards-new] setLastViewedCardgroup failed", { cardgroupId: currentId, err });
    });
    router.push(returnTo ?? `/cardgroups/${currentId}/cards`);
  }

  async function handleCreate(values: { front: string; back: string }) {
    if (!currentId) return;
    try {
      await createCard({
        variables: {
          input: { cardgroupId: currentId, front: values.front, back: values.back },
        },
      });
      markCreationSucceeded();
    } catch (err) {
      // Duplicate-front is routine validation, not a failure operators should
      // be paged for: open the overwrite dialog, set `duplicate` state, and skip
      // the [cards-new-client] console.error log. CardForm's banner stays empty
      // because the duplicate-front error is a field-level BAD_USER_INPUT and
      // getBackendErrorBanner returns undefined for that shape; any inline
      // front-field message is shadowed by the dialog that overlays the form.
      const dupe = tryGetDuplicateCardInfo(err);
      if (dupe) {
        setDuplicate({
          existingCardId: dupe.existingCardId,
          existingBack: dupe.existingBack,
          attemptedFront: values.front,
          attemptedBack: values.back,
        });
        return;
      }
      console.error("[cards-new-client] create card rejection", {
        message: err instanceof Error ? err.message : String(err),
        err,
      });
    }
  }

  async function handleOverwrite() {
    if (!duplicate || !currentId) return;
    setOverwriteError(null);
    try {
      await updateCard({
        variables: {
          id: duplicate.existingCardId,
          input: { back: duplicate.attemptedBack },
        },
      });
      setDuplicate(null);
      markCreationSucceeded();
    } catch (err) {
      // Leave `duplicate` set so the dialog stays mounted; surface the failure
      // inline. Field-level error on `back` is the only validation-failure shape
      // updateCard can return today (the input only carries `back`); surface it
      // directly since getBackendErrorBanner skips field-level BAD_USER_INPUT
      // and would otherwise fall through to the generic banner fallback.
      const fieldErrors = getBackendFieldErrors(err);
      const banner = getBackendErrorBanner(err);
      const message = fieldErrors.back ?? banner ?? "Overwrite failed. Please try again.";
      setOverwriteError(message);
      console.error("[cards-new-client] overwrite card rejection", {
        message: err instanceof Error ? err.message : String(err),
        err,
      });
    }
  }

  function handleCancelOverwrite() {
    setDuplicate(null);
    setOverwriteError(null);
  }

  function handlePickerSelect(newId: string) {
    // Preserve the sanitized ?return= param so the post-create navigation
    // intent survives a cardgroup switch. Only the already-sanitized returnTo
    // value is re-encoded here — raw searchParams.get("return") is never used
    // directly (see sanitizeReturnTo call above).
    const newUrl =
      returnTo !== null
        ? `/cards/new?cardgroup=${encodeURIComponent(newId)}&return=${encodeURIComponent(returnTo)}`
        : `/cards/new?cardgroup=${encodeURIComponent(newId)}`;
    router.replace(newUrl, { scroll: false });
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <span className="text-sm font-medium text-muted-foreground">Cardgroup</span>
        <CardgroupChip name={currentName} onChangeRequested={() => setPickerOpen(true)} />
      </div>

      <CardgroupPickerSheet
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        selectedId={currentId}
        onSelect={handlePickerSelect}
        createReturnTo="/cards/new"
      />

      {currentId != null ? (
        <CardForm
          mode="create"
          idPrefix="cards-new-"
          defaultValues={{ front: "", back: "" }}
          submit={handleCreate}
          submitLabel="Add card"
          submitting={creating}
          error={createError}
        />
      ) : (
        <p className="text-sm text-muted-foreground">Select a cardgroup above to add a card.</p>
      )}

      {currentId != null && (
        <div className="flex justify-end pt-4 border-t">
          <Link
            href={`/cardgroups/${currentId}/cards`}
            className="text-sm text-muted-foreground hover:underline"
          >
            Done
          </Link>
        </div>
      )}

      {duplicate !== null && (
        <DuplicateOverwriteDialog
          duplicate={duplicate}
          onConfirm={handleOverwrite}
          onCancel={handleCancelOverwrite}
          loading={overwriting}
          error={overwriteError}
        />
      )}
    </div>
  );
}
