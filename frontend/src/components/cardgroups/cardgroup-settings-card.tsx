"use client";

import { useMutation } from "@apollo/client/react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import {
  CARDGROUPS_DEFAULT_VARS,
  DeleteCardgroupMutation,
  UpdateCardgroupMutation,
} from "@/app/cardgroups/queries";
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
import { MyCardgroupsConnectionDocument, MyCardgroupsDocument } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";

type Props = {
  cardgroup: { id: string; name: string };
};

export function CardgroupSettingsCard({ cardgroup }: Props) {
  const router = useRouter();
  const [dialogOpen, setDialogOpen] = useState(false);

  const [updateCardgroup, { loading: updating, error: updateError }] =
    useMutation(UpdateCardgroupMutation);

  const [deleteCardgroup, { loading: deleting, error: deleteError }] =
    useMutation(DeleteCardgroupMutation);

  const mutating = updating || deleting;
  const deleteBannerError = getBackendErrorBanner(deleteError);

  async function handleSave(values: { name: string }) {
    const result = await updateCardgroup({
      variables: { id: cardgroup.id, input: { name: values.name } },
    }).catch((err) => {
      console.error("[CardgroupSettingsCard] update rejection", err);
      return null;
    });

    if (result?.data?.updateCardgroup?.cardgroup) {
      // Stay on the management screen — refresh the RSC tree so the page header
      // (h1 with the cardgroup name) reflects the new value on the next paint.
      router.refresh();
    }
  }

  async function handleDelete() {
    const result = await deleteCardgroup({
      variables: { id: cardgroup.id },
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
                  (edge) => edge.node.id !== cardgroup.id,
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
              myCardgroups: existingFlat.myCardgroups.filter((cg) => cg.id !== cardgroup.id),
            },
          });
        }

        cache.evict({
          id: cache.identify({ __typename: "Cardgroup", id: cardgroup.id }),
        });
        cache.gc();
      },
    }).catch((err) => {
      console.error("[CardgroupSettingsCard] delete rejection", err);
      return null;
    });

    if (result?.data?.deleteCardgroup === true) {
      setDialogOpen(false);
      router.push("/cardgroups");
      router.refresh();
    }
  }

  return (
    <div className="space-y-6">
      <section className="rounded-lg border border-border bg-card p-6">
        <h2 className="mb-4 text-sm font-medium uppercase tracking-wide text-muted-foreground">
          Settings
        </h2>
        <CardgroupForm
          mode="edit"
          defaultValues={{ name: cardgroup.name }}
          submit={handleSave}
          submitting={mutating}
          error={updateError}
        />
      </section>

      <section
        aria-labelledby="danger-zone-heading"
        className="rounded-lg border border-destructive/40 bg-destructive/5 p-6"
      >
        <h2
          id="danger-zone-heading"
          className="mb-2 text-sm font-medium uppercase tracking-wide text-destructive"
        >
          Danger zone
        </h2>
        <p className="mb-4 text-sm text-muted-foreground">
          Deleting this cardgroup permanently removes it and all of its cards. This cannot be
          undone.
        </p>
        <AlertDialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <AlertDialogTrigger asChild>
            <Button type="button" variant="destructive" disabled={mutating}>
              Delete cardgroup
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete cardgroup</AlertDialogTitle>
              <AlertDialogDescription>
                {`This will permanently delete "${cardgroup.name}" and all its cards. This cannot be undone.`}
              </AlertDialogDescription>
            </AlertDialogHeader>
            {deleteBannerError && (
              <div
                className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
                role="alert"
              >
                {deleteBannerError}
              </div>
            )}
            <AlertDialogFooter>
              <AlertDialogCancel disabled={mutating}>Cancel</AlertDialogCancel>
              <AlertDialogAction
                className="bg-destructive text-destructive-foreground"
                disabled={mutating}
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
      </section>
    </div>
  );
}
