"use client";

import { Import, Layers, Play, Plus } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { type ReactNode, useEffect, useState } from "react";
import { toast } from "sonner";
import {
  type CardConnectionPageInfo,
  type CardEdge,
  CardsClient,
} from "@/app/cardgroups/[id]/cards/cards-client";
import { MergeFromCatalogSheet } from "@/components/cardgroups/merge-from-catalog-sheet";
import { Button } from "@/components/ui/button";
import { SplitButtonMenu } from "@/components/ui/split-button-menu";
import { FLAMINGO_EVENT, subscribeFlamingo } from "@/lib/events/flamingo-events";

type Props = {
  cardgroupId: string;
  cardgroupName: string;
  initialEdges: CardEdge[];
  initialPageInfo: CardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Optional render prop that lets the parent render a page-level header with
   * the live totalCount sourced from the Apollo cache. When provided, the
   * render prop receives `{ totalCount, onBatchImport, onMerge }` and is
   * invoked above the toolbar row. `onBatchImport` and `onMerge` let the
   * header's overflow menu host those actions on mobile (the standalone mobile
   * toolbar buttons were removed). When omitted, no page-level header is
   * rendered.
   */
  renderPageHeader?: (args: {
    totalCount: number;
    onBatchImport: () => void;
    onMerge: () => void;
  }) => ReactNode;
};

export function CardgroupCardsSection({
  cardgroupId,
  cardgroupName,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
  renderPageHeader,
}: Props) {
  const learnHref = `/learn/${encodeURIComponent(cardgroupId)}`;
  const t = useTranslations("Cardgroups");
  const [mergeOpen, setMergeOpen] = useState(false);
  const onMerge = () => setMergeOpen(true);

  useEffect(() => {
    return subscribeFlamingo(FLAMINGO_EVENT.merge, (detail) => {
      if (detail?.ownerId !== cardgroupId) return;
      setMergeOpen(true);
    });
  }, [cardgroupId]);

  // The render-prop form lets CardsClient pass its live totalCount (read from
  // Apollo cache, kept in sync with delete/bulk-delete/fetchMore) into the
  // heading and any parent-level header without running a duplicate useQuery.
  const renderHeader = ({
    totalCount,
    onAddCard,
    onBatchImport,
  }: {
    totalCount: number;
    onAddCard: () => void;
    onBatchImport: () => void;
  }) => (
    // The page-level header (title + count subtitle) renders first; the action
    // cluster sits on its own row below it, right-aligned on desktop. On mobile
    // the two also stack — header on top, full-width Start learning hero below.
    <div>
      {renderPageHeader?.({ totalCount, onBatchImport, onMerge })}
      <div className="mb-3 flex flex-col gap-2 md:flex-row md:items-center md:justify-end md:gap-2">
        {/* Primary CTA: full-width brand hero on mobile (h-11 = 44px tap target),
            compact on desktop. Add card on mobile is provided by the global "+"
            header action, so no Add button is duplicated here on small screens.
            Batch import on mobile lives in the page header's overflow menu. */}
        <Button asChild variant="brand" className="h-11 w-full px-8 md:h-9 md:w-auto md:px-4">
          <Link href={learnHref}>
            <Play aria-hidden="true" className="h-4 w-4" />
            {t("startLearning")}
          </Link>
        </Button>
        {/* Desktop split button: secondary Add card + dropdown with Batch import.
            Demoted to outline so Start learning is the single brand primary CTA. */}
        <div className="hidden md:inline-flex">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="rounded-r-none"
            onClick={onAddCard}
          >
            {t("addCardButton")}
          </Button>
          <SplitButtonMenu
            triggerLabel={t("addMoreOptions")}
            data-testid="cardgroup-add-more-options"
            items={[
              {
                key: "add",
                icon: <Plus aria-hidden="true" className="h-4 w-4" />,
                label: t("addACard"),
                onSelect: onAddCard,
              },
              {
                key: "import",
                icon: <Import aria-hidden="true" className="h-4 w-4" />,
                label: t("batchImport"),
                onSelect: onBatchImport,
                "data-testid": "cardgroup-batch-import-menuitem",
              },
              {
                key: "merge",
                icon: <Layers aria-hidden="true" className="h-4 w-4" />,
                label: t("mergeFromCatalog"),
                onSelect: onMerge,
                "data-testid": "cardgroup-merge-menuitem",
              },
            ]}
          />
        </div>
      </div>
    </div>
  );

  return (
    <>
      <CardsClient
        cardgroupId={cardgroupId}
        cardgroupName={cardgroupName}
        initialEdges={initialEdges}
        initialPageInfo={initialPageInfo}
        initialTotalCount={initialTotalCount}
        sectionHeader={renderHeader}
      />
      <MergeFromCatalogSheet
        open={mergeOpen}
        onOpenChange={setMergeOpen}
        targetCardgroupId={cardgroupId}
        targetCardgroupName={cardgroupName}
        onMerged={({ addedCount, updatedCount }) => {
          toast(t("mergeSuccess", { added: addedCount, updated: updatedCount }));
        }}
      />
    </>
  );
}
