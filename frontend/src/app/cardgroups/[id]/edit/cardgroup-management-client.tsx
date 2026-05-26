"use client";

import Link from "next/link";
import type { CardConnectionPageInfo, CardEdge } from "@/app/cardgroups/[id]/cards/cards-client";
import { CardgroupCardsSection } from "@/components/cardgroups/cardgroup-cards-section";
import { CardgroupHeader } from "@/components/cardgroups/cardgroup-header";

type Props = {
  cardgroup: { id: string; name: string };
  initialEdges: CardEdge[];
  initialPageInfo: CardConnectionPageInfo;
  initialTotalCount: number;
};

export function CardgroupManagementClient({
  cardgroup,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
}: Props) {
  return (
    <main className="p-4 md:p-8">
      <div className="mb-4">
        <Link href="/cardgroups" className="text-sm text-muted-foreground hover:underline">
          &larr; Cardgroups
        </Link>
      </div>

      <CardgroupCardsSection
        cardgroupId={cardgroup.id}
        cardgroupName={cardgroup.name}
        initialEdges={initialEdges}
        initialPageInfo={initialPageInfo}
        initialTotalCount={initialTotalCount}
        renderPageHeader={({ totalCount, onBatchImport }) => (
          <CardgroupHeader
            cardgroup={cardgroup}
            totalCount={totalCount}
            onBatchImport={onBatchImport}
          />
        )}
      />
    </main>
  );
}
