"use client";

import {
  type MasterCardConnectionPageInfo,
  type MasterCardEdge,
  MasterCardsClient,
} from "./cards/master-cards-client";

type MasterCardsSectionProps = {
  masterId: string;
  deckName: string;
  initialEdges: MasterCardEdge[];
  initialPageInfo: MasterCardConnectionPageInfo;
  initialTotalCount: number;
  onTotalCountChange: (count: number) => void;
};

export function MasterCardsSection({
  masterId,
  deckName,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
  onTotalCountChange,
}: MasterCardsSectionProps) {
  return (
    <section data-testid="master-cards-section" data-master-id={masterId}>
      <MasterCardsClient
        masterId={masterId}
        deckName={deckName}
        initialEdges={initialEdges}
        initialPageInfo={initialPageInfo}
        initialTotalCount={initialTotalCount}
        onTotalCountChange={onTotalCountChange}
      />
    </section>
  );
}
