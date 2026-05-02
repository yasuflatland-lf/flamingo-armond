"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { CreateCardMutation } from "@/app/cardgroups/queries";
import { CardForm } from "@/components/cardgroups/card-form";
import { CardgroupChip } from "@/components/cardgroups/cardgroup-chip";
import CardgroupPickerSheet from "@/components/cardgroups/cardgroup-picker-sheet";
import { useMutation } from "@apollo/client/react";

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

  // Derive the display name from the server-seeded list.
  const currentName =
    currentId != null
      ? (myCardgroups.find((cg) => cg.id === currentId)?.name ?? null)
      : null;

  const [createCard, { loading: creating, error: createError }] =
    useMutation(CreateCardMutation);

  async function handleCreate(values: { front: string; back: string }) {
    if (!currentId) return;
    await createCard({
      variables: {
        input: { cardgroupId: currentId, front: values.front, back: values.back },
      },
    })
      .then(() => {
        router.push(`/cardgroups/${currentId}/cards`);
      })
      .catch((err) => {
        console.error("[cards-new-client] create card rejection", err);
      });
  }

  function handlePickerSelect(newId: string) {
    router.replace(`/cards/new?cardgroup=${newId}`, { scroll: false });
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <span className="text-sm font-medium text-muted-foreground">Cardgroup</span>
        <CardgroupChip
          name={currentName}
          onChangeRequested={() => setPickerOpen(true)}
        />
      </div>

      <CardgroupPickerSheet
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        selectedId={currentId}
        onSelect={handlePickerSelect}
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
        <p className="text-sm text-muted-foreground">
          Select a cardgroup above to add a card.
        </p>
      )}
    </div>
  );
}
