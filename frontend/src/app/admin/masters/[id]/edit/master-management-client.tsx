"use client";

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
  return (
    <main className="p-4 md:p-8">
      {/* The header is rendered inside the cards section via the render-prop so
          its card count tracks the live Apollo-cache totalCount; batch import
          now lives in the cards toolbar (mobile-visible), not the deck menu —
          no count state is lifted into this component. */}
      <MasterCardsSection
        masterId={master.id}
        deckName={master.name}
        initialEdges={initialEdges}
        initialPageInfo={initialPageInfo}
        initialTotalCount={initialTotalCount}
        renderPageHeader={({ totalCount }) => (
          <MasterEditHeader master={master} cardCount={totalCount} />
        )}
      />
    </main>
  );
}
