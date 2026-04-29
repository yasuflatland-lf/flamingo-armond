"use client";

import { useMutation } from "@apollo/client/react";
import { useMemo, useState } from "react";
import {
  CreateCardMutation,
  DeleteCardMutation,
  UpdateCardMutation,
} from "@/app/cardgroups/queries";
import { CardForm } from "@/components/cardgroups/card-form";
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
import { CardsByCardgroupDocument, type CardsByCardgroupQuery } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";

type Card = CardsByCardgroupQuery["cardsByCardgroup"][number];

type Props = {
  cardgroupId: string;
  initialCards: Card[];
};

export function CardsClient({ cardgroupId, initialCards }: Props) {
  const [cards, setCards] = useState<Card[]>(initialCards);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [createFormKey, setCreateFormKey] = useState(0);

  const [createCard, { loading: creating, error: createError }] = useMutation(CreateCardMutation, {
    update(cache, { data }) {
      if (!data?.createCard?.card) return;
      const existing = cache.readQuery({
        query: CardsByCardgroupDocument,
        variables: { cardgroupId },
      });
      cache.writeQuery({
        query: CardsByCardgroupDocument,
        variables: { cardgroupId },
        data: {
          cardsByCardgroup: [...(existing?.cardsByCardgroup ?? []), data.createCard.card],
        },
      });
      setCards((prev) => [...prev, data.createCard.card]);
      setCreateFormKey((k) => k + 1);
    },
  });

  const [updateCard, { loading: updating, error: updateError }] = useMutation(UpdateCardMutation, {
    update(_cache, { data }) {
      if (!data?.updateCard?.card) return;
      const updated = data.updateCard.card;
      setCards((prev) => prev.map((c) => (c.id === updated.id ? updated : c)));
    },
  });

  const [deleteCard, { error: deleteError }] = useMutation(DeleteCardMutation, {
    update(cache, { data }, { variables }) {
      if (!data?.deleteCard) return;
      const id = variables?.id as string | undefined;
      if (!id) return;
      cache.evict({ id: cache.identify({ __typename: "Card", id }) });
      cache.gc();
      setCards((prev) => prev.filter((c) => c.id !== id));
    },
  });

  const deleteBannerError = useMemo(() => getBackendErrorBanner(deleteError), [deleteError]);

  async function handleCreate(values: { front: string; back: string }) {
    await createCard({
      variables: { input: { cardgroupId, front: values.front, back: values.back } },
    }).catch((err) => {
      console.error("[CardsClient] create rejection", err);
    });
  }

  async function handleUpdate(id: string, values: { front: string; back: string }) {
    const result = await updateCard({
      variables: { id, input: { front: values.front, back: values.back } },
    }).catch((err) => {
      console.error("[CardsClient] update rejection", err);
      return null;
    });
    if (result?.data?.updateCard?.card) {
      setEditingId(null);
    }
  }

  return (
    <div className="space-y-6">
      {deleteBannerError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {deleteBannerError}
        </div>
      )}

      <section>
        <h2 className="mb-3 text-sm font-medium uppercase tracking-wide text-muted-foreground">
          Add a card
        </h2>
        <CardForm
          key={createFormKey}
          mode="create"
          idPrefix="add-"
          defaultValues={{ front: "", back: "" }}
          submit={handleCreate}
          submitLabel="Add"
          submitting={creating}
          error={createError}
        />
      </section>

      <section>
        <h2 className="mb-3 text-sm font-medium uppercase tracking-wide text-muted-foreground">
          Cards ({cards.length})
        </h2>

        {cards.length === 0 ? (
          <p className="text-sm text-muted-foreground">No cards yet. Add one above.</p>
        ) : (
          <ul className="space-y-3">
            {cards.map((card) =>
              editingId === card.id ? (
                <li key={card.id} className="rounded-md border border-border p-4">
                  <CardForm
                    mode="edit"
                    idPrefix={`edit-${card.id}-`}
                    defaultValues={{ front: card.front, back: card.back }}
                    submit={(values) => handleUpdate(card.id, values)}
                    submitLabel="Save"
                    submitting={updating}
                    error={updateError}
                    onCancel={() => setEditingId(null)}
                  />
                </li>
              ) : (
                <li
                  key={card.id}
                  className="flex items-start justify-between gap-4 rounded-md border border-border px-4 py-3"
                >
                  <div className="min-w-0 flex-1 space-y-1">
                    <p className="text-sm font-medium">{card.front}</p>
                    <p className="text-sm text-muted-foreground">{card.back}</p>
                  </div>
                  <div className="flex shrink-0 gap-2">
                    <Button variant="outline" size="sm" onClick={() => setEditingId(card.id)}>
                      Edit
                    </Button>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="destructive" size="sm">
                          Delete
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Delete card?</AlertDialogTitle>
                          <AlertDialogDescription>
                            This action cannot be undone.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction
                            onClick={async () => {
                              await deleteCard({ variables: { id: card.id } }).catch(console.error);
                            }}
                          >
                            Delete
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                </li>
              ),
            )}
          </ul>
        )}
      </section>
    </div>
  );
}
