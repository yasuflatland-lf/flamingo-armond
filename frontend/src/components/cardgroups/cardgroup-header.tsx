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
        {/* Separator divides the constructive actions (rename, import) from the
            destructive delete. Always shown now that rename leads the menu. */}
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
      {/*
        Single-DOM responsive header — back, count, title, and the overflow
        menu are each rendered once and repositioned via CSS grid template
        areas, so the layout reflows without duplicating any element (the test
        contract expects exactly one back link, one count, one h1, and one
        overflow trigger).

        Mobile (`< md`): app-bar row — inline back (left), card count centered,
        overflow menu (right) — with the title centered on its own row below.
        The 1fr/auto/1fr columns keep the count truly centered regardless of
        the back/overflow cluster widths.

        Desktop (`>= md`): title left with the card count as a muted subtitle
        beneath it; back stays on the leading edge and the overflow trigger is
        pushed to the trailing edge of the title row. The page-level action
        buttons that `CardgroupCardsSection` lays out now sit on their own row
        below this header. */}
      <header className="mb-6 grid grid-cols-[1fr_auto_1fr] items-center gap-x-2 gap-y-1 [grid-template-areas:'back_count_overflow'_'title_title_title'] md:grid-cols-[auto_minmax(0,1fr)_auto] md:gap-y-0 md:[grid-template-areas:'back_title_overflow'_'back_count_overflow']">
        <Link
          href="/cardgroups"
          className="shrink-0 justify-self-start [grid-area:back] rounded-sm text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <ChevronLeft aria-hidden="true" className="h-5 w-5" />
          <span className="sr-only">{t("backLink")}</span>
        </Link>
        <h1 className="min-w-0 truncate [grid-area:title] text-center text-2xl font-semibold leading-tight md:text-left">
          {cardgroup.name}
        </h1>
        <p className="justify-self-center [grid-area:count] text-sm text-muted-foreground md:justify-self-start">
          {t("cardsCount", { count: totalCount })}
        </p>
        <div className="justify-self-end [grid-area:overflow]">{actions}</div>
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
