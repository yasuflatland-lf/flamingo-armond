"use client";

import { Trash2, X } from "lucide-react";
import { useTranslations } from "next-intl";
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
import { Button, buttonVariants } from "@/components/ui/button";

export type BulkActionBarProps = {
  count: number;
  busy: boolean;
  onConfirm: () => void;
  onClear: () => void;
};

export function BulkActionBar({ count, busy, onConfirm, onClear }: BulkActionBarProps) {
  const t = useTranslations("Cards");
  const tCommon = useTranslations("Common");

  return (
    <div
      className="mb-3 flex flex-col gap-2 rounded-md border border-border bg-muted/50 px-3 py-3 sm:flex-row sm:items-center sm:gap-3 sm:px-4 sm:py-2"
      data-testid="cards-bulk-action-bar"
    >
      <span className="text-center text-sm font-medium sm:flex-1 sm:text-left">
        {t("selectedCount", { count })}
      </span>
      <AlertDialog>
        <AlertDialogTrigger asChild>
          <Button
            variant="destructive"
            size="sm"
            disabled={busy}
            data-testid="cards-bulk-delete-button"
            className="w-full sm:w-auto"
          >
            {tCommon("deleteSelected")}
            <Trash2 aria-hidden="true" className="ml-1.5 h-4 w-4" />
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteCardsTitle", { count })}</AlertDialogTitle>
            <AlertDialogDescription>{tCommon("cannotBeUndone")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tCommon("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              className={buttonVariants({ variant: "destructive" })}
              data-testid="cards-bulk-confirm"
              onClick={onConfirm}
            >
              {t("deleteConfirmCount", { count })}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <Button variant="outline" size="sm" className="w-full sm:w-auto" onClick={onClear}>
        {tCommon("cancel")}
        <X aria-hidden="true" className="ml-1.5 h-4 w-4" />
      </Button>
    </div>
  );
}
