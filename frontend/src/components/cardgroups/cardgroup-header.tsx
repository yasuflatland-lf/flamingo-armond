"use client";

import { useMutation } from "@apollo/client/react";
import { Import, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useState } from "react";
import { CARDGROUPS_DEFAULT_VARS, DeleteCardgroupMutation } from "@/app/cardgroups/queries";
import { CardgroupRenameForm } from "@/components/cardgroups/cardgroup-rename-form";
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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { FormSheet } from "@/components/ui/form-sheet";
import { MyCardgroupsConnectionDocument } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";

type Props = {
  cardgroup: { id: string; name: string };
  totalCount: number;
  /** Called when the user selects "Batch import" from the mobile options menu. */
  onBatchImport?: () => void;
};

export function CardgroupHeader({ cardgroup, totalCount, onBatchImport }: Props) {
  const router = useRouter();
  const [renameOpen, setRenameOpen] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const [deleteCardgroup, { loading: deleting, error: deleteError }] =
    useMutation(DeleteCardgroupMutation);

  const deleteBannerError = getBackendErrorBanner(deleteError);

  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");

  const handleRenameSubmittingChange = useCallback((submitting: boolean) => {
    setRenaming(submitting);
  }, []);

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

        cache.evict({
          id: cache.identify({ __typename: "Cardgroup", id: cardgroup.id }),
        });
        cache.gc();
      },
    }).catch((err) => {
      console.error("[CardgroupHeader] delete rejection", err);
      return null;
    });

    if (result?.data?.deleteCardgroup === true) {
      setDeleteDialogOpen(false);
      router.push("/cardgroups");
      router.refresh();
    }
  }

  return (
    <>
      <div className="mb-6 flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold">{cardgroup.name}</h1>
        <Badge variant="secondary">{totalCount} cards</Badge>
        <div className="ml-auto">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label={t("cardgroupOptions")}>
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => setRenameOpen(true)} className="gap-2">
                <Pencil className="h-4 w-4" />
                {t("rename")}
              </DropdownMenuItem>
              {onBatchImport && (
                <>
                  <DropdownMenuSeparator />
                  {/* Batch import is shown here for mobile users;
                      the desktop split button (hidden md:inline-flex) covers desktop. */}
                  <DropdownMenuItem onSelect={onBatchImport} className="gap-2 md:hidden">
                    <Import className="h-4 w-4" />
                    {t("batchImport")}
                  </DropdownMenuItem>
                </>
              )}
              <DropdownMenuSeparator />
              {/* Future reserved items (not yet implemented):
                  <DropdownMenuItem disabled>Export to TextDic</DropdownMenuItem>
                  <DropdownMenuItem disabled>Duplicate</DropdownMenuItem>
                  <DropdownMenuSeparator />
              */}
              <DropdownMenuItem
                onSelect={() => setDeleteDialogOpen(true)}
                className="gap-2 text-destructive focus:text-destructive"
              >
                <Trash2 className="h-4 w-4" />
                {t("deleteCardgroupTitle")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <FormSheet
        open={renameOpen}
        onOpenChange={setRenameOpen}
        title={t("renameCardgroupTitle")}
        confirmOnDismiss={false}
        submitting={renaming}
        size="sm"
      >
        <CardgroupRenameForm
          cardgroup={cardgroup}
          onSaved={() => {
            setRenaming(false);
            setRenameOpen(false);
          }}
          onSubmittingChange={handleRenameSubmittingChange}
        />
      </FormSheet>

      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteCardgroupTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteCardgroupDesc", { name: cardgroup.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {deleteBannerError && (
            <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
              {deleteBannerError}
            </div>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>{tCommon("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground"
              disabled={deleting}
              onClick={(e) => {
                e.preventDefault();
                void handleDelete();
              }}
            >
              {tCommon("delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
