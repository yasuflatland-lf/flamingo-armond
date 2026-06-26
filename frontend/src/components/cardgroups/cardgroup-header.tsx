"use client";

import { Import, Layers, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useState } from "react";
import { toast } from "sonner";
import { CardgroupRenameForm } from "@/components/cardgroups/cardgroup-rename-form";
import { MergeFromCatalogSheet } from "@/components/cardgroups/merge-from-catalog-sheet";
import { useDeleteCardgroup } from "@/components/cardgroups/use-delete-cardgroup";
import { DetailPageHeader } from "@/components/nav/detail-page-header";
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
  /**
   * Opens the batch-import sheet (owned by `CardsClient`). Surfaced here so the
   * overflow menu can host batch import on mobile, where the standalone toolbar
   * button was removed. Required so the wire from `CardsClient` is enforced at
   * compile time rather than silently defaulting to a no-op.
   */
  onBatchImport: () => void;
};

export function CardgroupHeader({ cardgroup, totalCount, onBatchImport }: Props) {
  const router = useRouter();
  const [renameOpen, setRenameOpen] = useState(false);
  const [mergeOpen, setMergeOpen] = useState(false);
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

  const actions = (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t("cardgroupOptions")}>
          <MoreHorizontal className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {/* Rename — the only editable attribute of a cardgroup, hosted as the
            top menu item. */}
        <DropdownMenuItem
          onSelect={() => setRenameOpen(true)}
          className="gap-2"
          data-testid="cardgroup-rename-menuitem"
        >
          <Pencil className="h-4 w-4" />
          {t("rename")}
        </DropdownMenuItem>
        {/* Mobile-only: batch import lives here after the standalone toolbar
            button was removed. Hidden on desktop, where the cards toolbar's
            split-button menu still hosts batch import. */}
        <DropdownMenuItem
          onSelect={onBatchImport}
          className="gap-2 md:hidden"
          data-testid="cardgroup-import-menuitem"
        >
          <Import className="h-4 w-4" />
          {t("batchImport")}
        </DropdownMenuItem>
        {/* Merge from catalog — copies a published master deck into this
            cardgroup. A constructive action, so it sits before the separator and
            stays visible on all viewports. */}
        <DropdownMenuItem
          onSelect={() => setMergeOpen(true)}
          className="gap-2"
          data-testid="cardgroup-merge-menuitem"
        >
          <Layers className="h-4 w-4" />
          {t("mergeFromCatalog")}
        </DropdownMenuItem>
        {/* Separator divides the constructive actions (rename, import, merge)
            from the destructive delete. Always shown now that rename leads the
            menu. */}
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onSelect={() => setDeleteDialogOpen(true)}
          className="gap-2 text-destructive focus:text-destructive"
        >
          <Trash2 className="h-4 w-4" />
          {t("deleteCardgroupTitle")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );

  return (
    <>
      {/* Shared detail-page header: inline back (left), centered card-count
          meta, and the overflow menu as the trailing action cluster, with the
          deck name as the centered title one tier below. The 1fr/auto/1fr
          app-bar grid keeps the count truly centered regardless of the
          back/action cluster widths. */}
      <DetailPageHeader
        backHref="/cardgroups"
        backLabel={t("backLink")}
        title={cardgroup.name}
        meta={
          <p className="text-sm text-muted-foreground">{t("cardsCount", { count: totalCount })}</p>
        }
        actions={actions}
      />

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

      <MergeFromCatalogSheet
        open={mergeOpen}
        onOpenChange={setMergeOpen}
        targetCardgroupId={cardgroup.id}
        onMerged={({ addedCount, updatedCount }) => {
          toast(t("mergeSuccess", { added: addedCount, updated: updatedCount }));
        }}
      />

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
