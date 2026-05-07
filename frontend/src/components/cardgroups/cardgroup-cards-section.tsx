"use client";

import { Play, Plus } from "lucide-react";
import Link from "next/link";
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
};

export function CardgroupCardsSection({
  cardgroupId,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
}: Props) {
  const addCardHref = `/cards/new?cardgroup=${encodeURIComponent(
    cardgroupId,
  )}&return=/cardgroups/${encodeURIComponent(cardgroupId)}/edit`;
  const learnHref = `/learn/${encodeURIComponent(cardgroupId)}`;

  // The render-prop form lets CardsClient pass its live totalCount (read from
  // Apollo cache, kept in sync with delete/bulk-delete/fetchMore) into the
  // heading without the parent running a duplicate useQuery for the same key.
  const renderHeader = ({ totalCount }: { totalCount: number }) => (
    <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
      <h2 className="text-sm font-medium uppercase tracking-wide text-muted-foreground">
        Cards ({totalCount})
      </h2>
      <div className="flex gap-2">
        <Button asChild variant="outline" size="sm">
          <Link href={learnHref}>
            Start learning
            <Play aria-hidden="true" className="ml-1.5 h-4 w-4" />
          </Link>
        </Button>
        <Button asChild variant="brand" size="sm">
          <Link href={addCardHref}>
            Add card
            <Plus aria-hidden="true" className="ml-1.5 h-4 w-4" />
          </Link>
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
