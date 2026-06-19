"use client";

import { Import, Plus } from "lucide-react";
import { useTranslations } from "next-intl";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { SplitButtonMenu } from "@/components/ui/split-button-menu";
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
  /**
   * Renders the deck-level page header (title / status badge / overflow menu)
   * with the live `totalCount`. Omitted → no page header.
   */
  renderPageHeader?: (args: { totalCount: number }) => ReactNode;
};

export function MasterCardsSection({
  masterId,
  deckName,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
  renderPageHeader,
}: MasterCardsSectionProps) {
  const tCards = useTranslations("Cards");
  const tCardgroups = useTranslations("Cardgroups");

  // MasterCardsClient passes its live totalCount (read from the Apollo cache,
  // kept in sync with create / delete / fetchMore) into this render fn, so the
  // page header and the desktop toolbar stay current without a second useQuery.
  const renderHeader = ({
    totalCount,
    onAddCard,
    onBatchImport,
  }: {
    totalCount: number;
    onAddCard: () => void;
    onBatchImport: () => void;
  }) => (
    <div>
      {renderPageHeader?.({ totalCount })}
      {/* Mobile (<md): import lives here; Add card is the global header "+". */}
      <div className="mb-3 flex justify-end md:hidden">
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onBatchImport}
          data-testid="master-import-mobile"
        >
          <Import aria-hidden="true" className="h-4 w-4" />
          {tCardgroups("batchImport")}
        </Button>
      </div>
      {/* Desktop (md+): Add card + a dropdown that folds in Batch import. */}
      <div className="mb-3 hidden justify-end md:flex">
        <div className="inline-flex">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="rounded-r-none"
            onClick={onAddCard}
            data-testid="master-add-card"
          >
            <Plus aria-hidden="true" className="h-4 w-4" />
            {tCards("addCard")}
          </Button>
          <SplitButtonMenu
            triggerLabel={tCardgroups("addMoreOptions")}
            data-testid="master-add-more-options"
            items={[
              {
                key: "add",
                icon: <Plus aria-hidden="true" className="h-4 w-4" />,
                label: tCards("addCard"),
                onSelect: onAddCard,
              },
              {
                key: "import",
                icon: <Import aria-hidden="true" className="h-4 w-4" />,
                label: tCardgroups("batchImport"),
                onSelect: onBatchImport,
                "data-testid": "master-batch-import-menuitem",
              },
            ]}
          />
        </div>
      </div>
    </div>
  );

  return (
    <section data-testid="master-cards-section" data-master-id={masterId}>
      <MasterCardsClient
        masterId={masterId}
        deckName={deckName}
        initialEdges={initialEdges}
        initialPageInfo={initialPageInfo}
        initialTotalCount={initialTotalCount}
        sectionHeader={renderHeader}
      />
    </section>
  );
}
