"use client";

import { Import, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useState } from "react";
import { CardgroupRenameForm } from "@/components/cardgroups/cardgroup-rename-form";
import { useDeleteCardgroup } from "@/components/cardgroups/use-delete-cardgroup";
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
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ErrorBanner } from "@/components/ui/error-banner";
import { FormSheet } from "@/components/ui/form-sheet";
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

  const { deleteCardgroup, deleting, error: deleteError } = useDeleteCardgroup();

  const deleteBannerError = getBackendErrorBanner(deleteError);

  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");

  const handleRenameSubmittingChange = useCallback((submitting: boolean) => {
    setRenaming(submitting);
  }, []);

  async function handleDelete() {
    const deleted = await deleteCardgroup(cardgroup.id);
    if (deleted) {
      setDeleteDialogOpen(false);
      router.push("/cardgroups");
      router.refresh();
    }
  }

  return (
    <>
      <div className="mb-6 flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-semibold leading-tight break-words">{cardgroup.name}</h1>
          {/* Count demoted from a Badge to muted metadata so the title is the
              unambiguous anchor of the header. */}
          <p className="mt-1 text-sm text-muted-foreground">
            {t("cardsCount", { count: totalCount })}
          </p>
        </div>
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
          {deleteBannerError && <ErrorBanner>{deleteBannerError}</ErrorBanner>}
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
