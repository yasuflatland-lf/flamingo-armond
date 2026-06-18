"use client";

import { ChevronLeft } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("Cardgroups");
  return (
    <main className="p-4 md:p-8">
      {/* Compact breadcrumb: tight to the title below so the back link reads as
          a sibling of the header, not a floating standalone row. */}
      <div className="mb-2">
        <Link
          href="/cardgroups"
          className="inline-flex items-center text-muted-foreground hover:text-foreground"
        >
          <ChevronLeft aria-hidden="true" className="h-5 w-5" />
          <span className="sr-only">{t("backLink")}</span>
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
