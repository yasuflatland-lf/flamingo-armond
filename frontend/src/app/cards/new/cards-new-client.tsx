"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { CreateCardMutation } from "@/app/cardgroups/queries";
import { SetLastViewedCardgroupMutation } from "@/app/learn/queries";
import { CardForm } from "@/components/cardgroups/card-form";
import { CardgroupChip } from "@/components/cardgroups/cardgroup-chip";
import CardgroupPickerSheet from "@/components/cardgroups/cardgroup-picker-sheet";

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

  const resetFormRef = useRef<(() => void) | null>(null);
  // `successKey` doubles as "is the indicator visible?" (null = hidden) and as
  // a remount key — bumping it on each successful submit forces SuccessIndicator
  // to remount and restart its 2 s timer.
  const [successKey, setSuccessKey] = useState<number | null>(null);
  const [lastAddedName, setLastAddedName] = useState<string | null>(null);

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
      console.error("[cards-new-client] create card rejection", {
        message: err instanceof Error ? err.message : String(err),
        err,
      });
    }
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
    </div>
  );
}
