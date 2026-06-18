"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { MasterCardsSection } from "./master-cards-section";
import { MasterEditHeader } from "./master-edit-header";
import type { AdminMasterDeck } from "./queries";

type Props = { master: AdminMasterDeck };

const EMPTY_PAGE_INFO = {
  __typename: "PageInfo" as const,
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: null,
  endCursor: null,
};

export function MasterManagementClient({ master }: Props) {
  const t = useTranslations("AdminMasters");
  const [liveCount, setLiveCount] = useState(master.cardCount);
  return (
    <main className="p-4 md:p-8">
      <div className="mb-2">
        <Link
          href="/admin/masters"
          className="inline-flex text-sm text-muted-foreground hover:text-foreground hover:underline"
        >
          {t("backToList")}
        </Link>
      </div>

      <MasterEditHeader master={master} cardCount={liveCount} />
      <MasterCardsSection
        masterId={master.id}
        deckName={master.name}
        initialEdges={[]}
        initialPageInfo={EMPTY_PAGE_INFO}
        initialTotalCount={master.cardCount}
        onTotalCountChange={setLiveCount}
      />
    </main>
  );
}
