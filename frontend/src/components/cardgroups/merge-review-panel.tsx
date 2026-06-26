"use client";

import { ChevronLeft } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button, buttonVariants } from "@/components/ui/button";

export type MergeReviewPanelProps = {
  catalogName: string;
  destName: string;
  addedCount: number;
  updatedCount: number;
  merging: boolean;
  onConfirm: () => void;
  onBack: () => void;
};

/**
 * In-sheet review state for the merge flow. Shows the From -> Into direction
 * and the dry-run diff (New / Updated) before committing. The confirm button
 * is destructive (red): merge overwrites matching cards' answers, a data-loss
 * action per the design-system convention.
 */
export function MergeReviewPanel({
  catalogName,
  destName,
  addedCount,
  updatedCount,
  merging,
  onConfirm,
  onBack,
}: MergeReviewPanelProps) {
  const t = useTranslations("Cardgroups");
  const total = addedCount + updatedCount;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="icon"
          onClick={onBack}
          disabled={merging}
          aria-label={t("mergeBack")}
          data-testid="merge-review-back"
        >
          <ChevronLeft className="h-4 w-4" />
        </Button>
        <h3 className="text-base font-semibold">{t("mergeReviewTitle")}</h3>
      </div>

      <div className="flex items-center gap-3 rounded-lg border border-border p-3">
        <div className="min-w-0 flex-1">
          <p className="text-xs uppercase tracking-wide text-muted-foreground">
            {t("mergeFromLabel")}
          </p>
          <p className="truncate text-sm font-medium">{catalogName}</p>
        </div>
        <span aria-hidden className="text-muted-foreground">
          →
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-xs uppercase tracking-wide text-muted-foreground">
            {t("mergeIntoLabel")}
          </p>
          <p className="truncate text-sm font-medium">{destName}</p>
        </div>
      </div>

      <div className="flex gap-3">
        <div className="flex-1 rounded-lg border border-border p-3 text-center">
          <p
            className="text-2xl font-bold tabular-nums text-success"
            data-testid="merge-review-added"
          >
            +{addedCount}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">{t("mergeDiffNew")}</p>
        </div>
        <div className="flex-1 rounded-lg border border-border p-3 text-center">
          <p
            className="text-2xl font-bold tabular-nums text-amber-600"
            data-testid="merge-review-updated"
          >
            {updatedCount}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">{t("mergeDiffUpdated")}</p>
        </div>
      </div>

      <div className="rounded-md bg-muted/50 p-3 text-xs text-muted-foreground">
        <p>✓ {t("mergeKeepNote")}</p>
        <p className="mt-1">⚠ {t("mergeOverwriteNote")}</p>
      </div>

      <button
        type="button"
        onClick={onConfirm}
        disabled={merging}
        className={buttonVariants({ variant: "destructive" })}
        data-testid="merge-review-confirm"
      >
        {t("mergeConfirmCount", { count: total })}
      </button>
    </div>
  );
}
