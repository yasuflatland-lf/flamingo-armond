"use client";

import Link from "next/link";
import type { CardConnectionPageInfo, CardEdge } from "@/app/cardgroups/[id]/cards/cards-client";
import { CardgroupCardsSection } from "@/components/cardgroups/cardgroup-cards-section";
import { CardgroupSettingsCard } from "@/components/cardgroups/cardgroup-settings-card";

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
      <div className="mb-6 flex items-center gap-4">
        <Link href="/cardgroups" className="text-sm text-muted-foreground hover:underline">
          &larr; Back
        </Link>
        <h1 className="text-2xl font-semibold">{cardgroup.name}</h1>
      </div>

      <div className="flex flex-col gap-6 md:flex-row md:items-start">
        <section className="order-1 min-w-0 flex-1 md:order-2">
          <CardgroupCardsSection
            cardgroupId={cardgroup.id}
            initialEdges={initialEdges}
            initialPageInfo={initialPageInfo}
            initialTotalCount={initialTotalCount}
          />
        </section>

        <aside className="order-2 w-full shrink-0 md:order-1 md:w-80">
          <CardgroupSettingsCard cardgroup={cardgroup} />
        </aside>
      </div>
    </main>
  );
}
