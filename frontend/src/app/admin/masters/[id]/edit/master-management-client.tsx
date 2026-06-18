"use client";

import { ChevronLeft } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useState } from "react";
import type { MasterCardConnectionPageInfo, MasterCardEdge } from "./cards/master-cards-client";
import { MasterCardsSection } from "./master-cards-section";
import { MasterEditHeader } from "./master-edit-header";
import type { AdminMasterDeck } from "./queries";

type Props = {
  master: AdminMasterDeck;
  initialEdges: MasterCardEdge[];
  initialPageInfo: MasterCardConnectionPageInfo;
  initialTotalCount: number;
};

export function MasterManagementClient({
  master,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
}: Props) {
  const t = useTranslations("AdminMasters");
  const [liveCount, setLiveCount] = useState(initialTotalCount);
  return (
    <main className="p-4 md:p-8">
      <div className="mb-2">
        <Link
          href="/admin/masters"
          className="inline-flex items-center text-muted-foreground hover:text-foreground"
        >
          <ChevronLeft aria-hidden="true" className="h-5 w-5" />
          <span className="sr-only">{t("backToList")}</span>
        </Link>
      </div>

      <MasterEditHeader master={master} cardCount={liveCount} />
      <MasterCardsSection
        masterId={master.id}
        deckName={master.name}
        initialEdges={initialEdges}
        initialPageInfo={initialPageInfo}
        initialTotalCount={initialTotalCount}
        onTotalCountChange={setLiveCount}
      />
    </main>
  );
}
