"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { DeleteCardgroupMutation, UpdateCardgroupMutation } from "@/app/cardgroups/queries";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
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

type Props = {
  cardgroup: { id: string; name: string };
};

export function EditCardgroupClient({ cardgroup }: Props) {
  const router = useRouter();
  const [dialogOpen, setDialogOpen] = useState(false);

  const [updateCardgroup, { loading: updating, error: updateError }] =
    useMutation(UpdateCardgroupMutation);

  const [deleteCardgroup, { loading: deleting, error: deleteError }] =
    useMutation(DeleteCardgroupMutation);

  const mutating = updating || deleting;
  const activeError = deleteError ?? updateError;

  async function handleSave(values: { name: string }) {
    await updateCardgroup({
      variables: { id: cardgroup.id, input: { name: values.name } },
    }).catch((err) => {
      console.error("[EditCardgroupClient] update rejection", err);
    });

    if (!updateError) {
      router.push(`/cardgroups/${cardgroup.id}`);
    }
  }

  async function handleDelete() {
    await deleteCardgroup({
      variables: { id: cardgroup.id },
      update(cache) {
        cache.evict({
          id: cache.identify({ __typename: "Cardgroup", id: cardgroup.id }),
        });
        cache.gc();
      },
    }).catch((err) => {
      console.error("[EditCardgroupClient] delete rejection", err);
    });

    if (!deleteError) {
      router.push("/cardgroups");
      router.refresh();
    }
  }

  const deleteButton = (
    <AlertDialog open={dialogOpen} onOpenChange={setDialogOpen}>
      <AlertDialogTrigger asChild>
        <Button type="button" variant="destructive" disabled={mutating}>
          Delete
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete cardgroup</AlertDialogTitle>
          <AlertDialogDescription>
            {`This will permanently delete "${cardgroup.name}" and all its cards. This cannot be undone.`}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={mutating}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            className="bg-destructive text-destructive-foreground"
            disabled={mutating}
            onClick={(e) => {
              e.preventDefault();
              void handleDelete();
              setDialogOpen(false);
            }}
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );

  return (
    <main className="mx-auto max-w-xl p-8">
      <div className="mb-6 flex items-center gap-4">
        <Link
          href={`/cardgroups/${cardgroup.id}`}
          className="text-sm text-muted-foreground hover:underline"
        >
          &larr; Back
        </Link>
        <h1 className="text-2xl font-semibold">Edit cardgroup</h1>
      </div>
      <CardgroupForm
        mode="edit"
        defaultValues={{ name: cardgroup.name }}
        submit={handleSave}
        submitting={mutating}
        error={activeError}
        secondarySlot={deleteButton}
      />
    </main>
  );
}
