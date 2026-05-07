"use client";

import { useMutation } from "@apollo/client/react";
import { Trash2 } from "lucide-react";
import { useState } from "react";
import { CARDGROUPS_DEFAULT_VARS, DeleteCardgroupMutation } from "@/app/cardgroups/queries";
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
import { MyCardgroupsConnectionDocument, MyCardgroupsDocument } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";

type Props = {
  id: string;
  name: string;
};

export function DeleteCardgroupButton({ id, name }: Props) {
  const [open, setOpen] = useState(false);

  const [deleteCardgroup, { loading: deleting, error: deleteError, reset }] = useMutation(
    DeleteCardgroupMutation,
    {
      update(cache, { data }) {
        if (!data?.deleteCardgroup) return;

        const existingConnection = cache.readQuery({
          query: MyCardgroupsConnectionDocument,
          variables: CARDGROUPS_DEFAULT_VARS,
        });
        if (existingConnection) {
          cache.writeQuery({
            query: MyCardgroupsConnectionDocument,
            variables: CARDGROUPS_DEFAULT_VARS,
            data: {
              myCardgroupsConnection: {
                ...existingConnection.myCardgroupsConnection,
                edges: existingConnection.myCardgroupsConnection.edges.filter(
                  (edge) => edge.node.id !== id,
                ),
                totalCount: Math.max(0, existingConnection.myCardgroupsConnection.totalCount - 1),
              },
            },
          });
        }

        const existingFlat = cache.readQuery({ query: MyCardgroupsDocument });
        if (existingFlat) {
          cache.writeQuery({
            query: MyCardgroupsDocument,
            data: {
              myCardgroups: existingFlat.myCardgroups.filter((cg) => cg.id !== id),
            },
          });
        }

        cache.evict({ id: cache.identify({ __typename: "Cardgroup", id }) });
        cache.gc();
      },
    },
  );

  async function handleDelete() {
    const result = await deleteCardgroup({ variables: { id } }).catch((err) => {
      console.error("[delete-cardgroup-button] delete rejection", { id, err });
      return null;
    });

    if (result?.data?.deleteCardgroup === true) {
      setOpen(false);
    }
  }

  const bannerError = getBackendErrorBanner(deleteError);

  function handleOpenChange(next: boolean) {
    if (!next && deleting) return;
    setOpen(next);
    if (!next) reset();
  }

  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
      <AlertDialogTrigger asChild>
        <Button type="button" variant="ghost" size="icon" aria-label={`Delete cardgroup ${name}`}>
          <Trash2 className="h-4 w-4" />
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete cardgroup</AlertDialogTitle>
          <AlertDialogDescription>
            {`This will permanently delete "${name}" and all its cards. This cannot be undone.`}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {bannerError && (
          <p role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {bannerError}
          </p>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            disabled={deleting}
            onClick={(e) => {
              e.preventDefault();
              void handleDelete();
            }}
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
