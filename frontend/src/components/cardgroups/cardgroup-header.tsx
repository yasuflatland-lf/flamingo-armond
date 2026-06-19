"use client";

import { ChevronLeft, Import, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import Link from "next/link";
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
        {/* Rename now lives on the title-row pencil; the overflow menu is
            lifecycle-only. The separator divides the mobile-only import item
            from delete, so it is also md:hidden — on desktop delete is the
            sole item and a leading separator would be a stray rule. */}
        <DropdownMenuSeparator className="md:hidden" />
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
      <header className="mb-6">
        {/* Row 1 — app-bar: inline back (left), card count centered, overflow
            menu (right). The 1fr/auto/1fr grid keeps the count truly centered
            regardless of the back/overflow cluster widths. */}
        <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
          <Link
            href="/cardgroups"
            className="shrink-0 justify-self-start rounded-sm text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <ChevronLeft aria-hidden="true" className="h-5 w-5" />
            <span className="sr-only">{t("backLink")}</span>
          </Link>
          <p className="justify-self-center text-sm text-muted-foreground">
            {t("cardsCount", { count: totalCount })}
          </p>
          <div className="justify-self-end">{actions}</div>
        </div>

        {/* Row 2 — editable title: centered name + pencil that opens the rename
            sheet (the only editable attribute of a cardgroup). */}
        <div className="mt-1 flex items-center justify-center gap-1.5">
          <h1 className="min-w-0 truncate text-2xl font-semibold leading-tight">
            {cardgroup.name}
          </h1>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="shrink-0 text-muted-foreground hover:text-foreground"
            aria-label={t("rename")}
            onClick={() => setRenameOpen(true)}
          >
            <Pencil className="h-5 w-5" />
          </Button>
        </div>
      </header>

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
