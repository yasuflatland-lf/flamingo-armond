"use client";

import { useTranslations } from "next-intl";
import type { ReactNode } from "react";
import { CardListScreen, type SectionHeaderArgs } from "@/components/cardgroups/card-list-screen";
import { CardgroupBatchImportForm } from "@/components/cardgroups/cardgroup-batch-import-form";
import type { CardsByCardgroupConnectionQuery } from "@/generated/graphql";
import { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import { type AddCardDetail, FLAMINGO_EVENT } from "@/lib/events/flamingo-events";
import { useCardMutations } from "./use-card-mutations";
import { useCardsConnection } from "./use-cards-connection";

type Connection = CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"];
export type CardEdge = Connection["edges"][number];
export type CardConnectionPageInfo = Connection["pageInfo"];

type Props = {
  cardgroupId: string;
  cardgroupName: string;
  initialEdges: CardEdge[];
  initialPageInfo: CardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Optional section header. When `undefined`, the default `<h2>Cards (n)</h2>`
   * is rendered. Pass a `ReactNode` to replace the header, `null` to suppress
   * it entirely, or a render function to access the live `totalCount` from
   * Apollo cache without spinning up a second `useQuery` in the parent.
   */
  sectionHeader?: ReactNode | ((args: SectionHeaderArgs) => ReactNode);
};

export function CardsClient({
  cardgroupId,
  cardgroupName,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
  sectionHeader,
}: Props) {
  const t = useTranslations("Cards");
  const search = useHeaderTakeoverSearch();

  const connection = useCardsConnection({
    cardgroupId,
    searchQuery: search.query,
    initialEdges,
    initialPageInfo,
    initialTotalCount,
    fetchMoreErrorMessage: t("fetchMoreFailed"),
  });

  const mutations = useCardMutations({ cardgroupId, queryVariables: connection.queryVariables });

  return (
    <CardListScreen
      search={search}
      connection={connection}
      mutations={mutations}
      ownerId={cardgroupId}
      addCardEvent={{
        name: FLAMINGO_EVENT.addCard,
        matches: (detail) => (detail as AddCardDetail | undefined)?.cardgroupId === cardgroupId,
      }}
      bulkDeleteLog={{ scope: "[CardsClient]", ownerKey: "cardgroupId" }}
      sectionHeader={sectionHeader}
      defaultHeader={({ totalCount }) => (
        <h2 className="mb-3 text-sm font-medium uppercase tracking-wide text-muted-foreground">
          {t("cardsCount", { count: totalCount })}
        </h2>
      )}
      importSheetTitle={
        // Object-first: header shows only the destination cardgroup (the high-risk
        // variable); the verb lives in the sr-only accessible name. Intentional
        // deviation from the verb-first sheets.
        <span
          className="block overflow-hidden text-ellipsis whitespace-nowrap"
          title={cardgroupName}
        >
          <span className="sr-only">{t("batchImportSrOnly")}</span>
          {cardgroupName}
        </span>
      }
      renderImportForm={({ onImported, onCancel }) => (
        <CardgroupBatchImportForm
          cardgroupId={cardgroupId}
          cardgroupName={cardgroupName}
          onImported={onImported}
          onCancel={onCancel}
        />
      )}
    />
  );
}
