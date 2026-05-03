"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
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
import { getBackendErrorBanner } from "@/lib/apollo/errors";
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

/**
 * Transient banner that announces a successful card creation. Self-dismisses
 * after 2 s. The parent assigns a fresh `key` on every successful submit so
 * consecutive adds remount this component and restart the timer instead of
 * silently extending the previous one.
 */
function SuccessIndicator({ message, onTimeout }: { message: string; onTimeout: () => void }) {
  useEffect(() => {
    const t = setTimeout(onTimeout, 2000);
    return () => clearTimeout(t);
  }, [onTimeout]);
  return (
    <div
      role="status"
      aria-live="polite"
      className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-900"
    >
      {message}
    </div>
  );
}

/**
 * Confirmation dialog for the duplicate-front overwrite flow. Renders a
 * side-by-side comparison of the existing card's back and the user's new
 * back so the choice is informed. User-facing strings are Japanese to match
 * the rest of the UX text in this app.
 */
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
          <AlertDialogTitle>カードはすでに存在します</AlertDialogTitle>
          <AlertDialogDescription>
            「{duplicate.attemptedFront}
            」というカードはすでにこのカードグループにあります。裏面を新しい内容で上書きしますか？学習履歴は維持されます。
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div className="space-y-1">
            <div className="text-xs font-medium text-muted-foreground">既存の裏面</div>
            <pre className="whitespace-pre-wrap break-words rounded-md border bg-muted/40 p-2 text-sm">
              {duplicate.existingBack}
            </pre>
          </div>
          <div className="space-y-1">
            <div className="text-xs font-medium text-muted-foreground">新しい裏面</div>
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
          <AlertDialogCancel disabled={loading}>キャンセル</AlertDialogCancel>
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
            {loading ? "上書き中…" : "上書き"}
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

  const currentName =
    currentId != null ? (myCardgroups.find((cg) => cg.id === currentId)?.name ?? null) : null;

  const [createCard, { loading: creating, error: createError }] = useMutation(CreateCardMutation);
  // Stay-on-page consecutive-add: do not carry an `optimisticResponse` for either
  // mutation. setLastViewed can fail typed (BAD_USER_INPUT when the cardgroup was
  // deleted between page render and submit) and Apollo v3.x does not roll back
  // optimistic writes on typed errors — see .claude/rules/pagination.md.
  const [setLastViewed] = useMutation(SetLastViewedCardgroupMutation);
  // Overwrite path also drops `optimisticResponse`: updateCard can fail typed
  // (UNAUTHENTICATED on session expiry, BAD_USER_INPUT from validators) and
  // Apollo v3.x does not roll back optimistic writes on typed errors.
  const [updateCard, { loading: overwriting }] = useMutation(UpdateCardMutation);

  const resetFormRef = useRef<(() => void) | null>(null);
  // `successKey` doubles as "is the indicator visible?" (null = hidden) and as
  // a remount key — bumping it on each successful submit forces SuccessIndicator
  // to remount and restart its 2 s timer.
  const [successKey, setSuccessKey] = useState<number | null>(null);
  const [lastAddedName, setLastAddedName] = useState<string | null>(null);
  const [duplicate, setDuplicate] = useState<DuplicateState>(null);
  // Inline error rendered inside DuplicateOverwriteDialog when updateCard fails.
  // Sibling state (rather than reusing createError) so the dialog stays open
  // and the message survives even after the create-mutation hook resets.
  const [overwriteError, setOverwriteError] = useState<string | null>(null);

  // Stable callback identities so that CardForm's useEffect([form, onResetReady])
  // and SuccessIndicator's useEffect([onTimeout]) do not re-fire on every parent
  // re-render (e.g. after the fire-and-forget setLastViewed mutation settles).
  const handleResetReady = useCallback((fn: () => void) => {
    resetFormRef.current = fn;
  }, []);

  const handleSuccessTimeout = useCallback(() => setSuccessKey(null), []);

  async function handleCreate(values: { front: string; back: string }) {
    if (!currentId) return;
    try {
      await createCard({
        variables: {
          input: { cardgroupId: currentId, front: values.front, back: values.back },
        },
      });
      // Fire-and-forget: this is the 4-priority-chain hint for the next visit
      // and must not block or fail the create flow. No `await` so a slow or
      // erroring persist call cannot delay the form reset.
      void setLastViewed({ variables: { cardgroupId: currentId } }).catch((err) => {
        console.warn("[cards-new] setLastViewedCardgroup failed", { cardgroupId: currentId, err });
      });
      resetFormRef.current?.();
      setLastAddedName(currentName);
      setSuccessKey(Date.now());
    } catch (err) {
      // Duplicate-front is a routine validation outcome, not a failure: surface
      // the overwrite dialog and skip both the generic error toast and the
      // console.error log. Anything else falls through to the existing path.
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
      // Mirror the create-success path: fire-and-forget the last-viewed hint,
      // reset the form, and bump the success indicator.
      void setLastViewed({ variables: { cardgroupId: currentId } }).catch((err) => {
        console.warn("[cards-new] setLastViewedCardgroup failed", { cardgroupId: currentId, err });
      });
      resetFormRef.current?.();
      setLastAddedName(currentName);
      setSuccessKey(Date.now());
    } catch (err) {
      // Leave `duplicate` set so the dialog stays mounted; surface the failure
      // inline. Use the same shape the form uses (getBackendErrorBanner) so the
      // message is familiar to the user.
      const banner = getBackendErrorBanner(err) ?? "上書きに失敗しました。もう一度お試しください。";
      setOverwriteError(banner);
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
    router.replace(`/cards/new?cardgroup=${newId}`, { scroll: false });
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

      {successKey !== null && lastAddedName && (
        <SuccessIndicator
          key={successKey}
          message={`✓ Card added to "${lastAddedName}"`}
          onTimeout={handleSuccessTimeout}
        />
      )}

      {currentId != null ? (
        <CardForm
          mode="create"
          idPrefix="cards-new-"
          defaultValues={{ front: "", back: "" }}
          submit={handleCreate}
          submitLabel="Add card"
          submitting={creating}
          error={createError}
          onResetReady={handleResetReady}
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
