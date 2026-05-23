"use client";

import { Play, Plus } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import {
  type CardConnectionPageInfo,
  type CardEdge,
  CardsClient,
} from "@/app/cardgroups/[id]/cards/cards-client";
import { Button } from "@/components/ui/button";

type Props = {
  cardgroupId: string;
  initialEdges: CardEdge[];
  initialPageInfo: CardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Optional render prop that lets the parent render a page-level header with
   * the live totalCount sourced from the Apollo cache. When provided, the
   * render prop receives `{ totalCount }` and is invoked above the toolbar row.
   * When omitted, no page-level header is rendered by this component.
   */
  renderPageHeader?: (args: { totalCount: number }) => ReactNode;
};

export function CardgroupCardsSection({
  cardgroupId,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
  renderPageHeader,
}: Props) {
  const learnHref = `/learn/${encodeURIComponent(cardgroupId)}`;

  // The render-prop form lets CardsClient pass its live totalCount (read from
  // Apollo cache, kept in sync with delete/bulk-delete/fetchMore) into the
  // heading and any parent-level header without running a duplicate useQuery.
  const renderHeader = ({
    totalCount,
    onAddCard,
  }: {
    totalCount: number;
    onAddCard: () => void;
  }) => (
    <div>
      {renderPageHeader ? renderPageHeader({ totalCount }) : null}
      <div className="mb-3 flex flex-wrap items-center justify-end gap-2">
        <Button asChild variant="outline" size="sm">
          <Link href={learnHref}>
            Start learning
            <Play aria-hidden="true" className="ml-1.5 h-4 w-4" />
          </Link>
        </Button>
        <Button type="button" variant="brand" size="sm" onClick={onAddCard}>
          Add card
          <Plus aria-hidden="true" className="ml-1.5 h-4 w-4" />
        </Button>
      </div>
    </div>
  );

  return (
    <CardsClient
      cardgroupId={cardgroupId}
      initialEdges={initialEdges}
      initialPageInfo={initialPageInfo}
      initialTotalCount={initialTotalCount}
      sectionHeader={renderHeader}
    />
  );
}
