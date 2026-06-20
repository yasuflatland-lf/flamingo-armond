"use client";

import { useTranslations } from "next-intl";
import type { ReactNode } from "react";
import { CardListScreen, type SectionHeaderArgs } from "@/components/cardgroups/card-list-screen";
import type { AdminMasterCardsConnectionQuery } from "@/generated/graphql";
import { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import { type AddMasterCardDetail, FLAMINGO_EVENT } from "@/lib/events/flamingo-events";
import { MasterBatchImportForm } from "./master-batch-import-form";
import { useMasterCardMutations } from "./use-master-card-mutations";
import { useMasterCardsConnection } from "./use-master-cards-connection";

type Connection = AdminMasterCardsConnectionQuery["adminMasterCardsConnection"];
export type MasterCardEdge = Connection["edges"][number];
export type MasterCardConnectionPageInfo = Connection["pageInfo"];

type Props = {
  masterId: string;
  deckName: string;
  initialEdges: MasterCardEdge[];
  initialPageInfo: MasterCardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Page-level header rendered above the search box. Pass a render function to
   * receive the live `totalCount` (sourced from the Apollo cache, kept in sync
   * with create/delete/fetchMore) plus the add-card and batch-import openers,
   * or a plain `ReactNode` to render as-is. Omitted → no header is rendered.
   */
  sectionHeader?: ReactNode | ((args: SectionHeaderArgs) => ReactNode);
};

export function MasterCardsClient({
  masterId,
  deckName,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
  sectionHeader,
}: Props) {
  const t = useTranslations("Cards");
  const search = useHeaderTakeoverSearch();

  const connection = useMasterCardsConnection({
    masterId,
    searchQuery: search.query,
    initialEdges,
    initialPageInfo,
    initialTotalCount,
    fetchMoreErrorMessage: t("fetchMoreFailed"),
  });

  const mutations = useMasterCardMutations({ masterId, queryVariables: connection.queryVariables });

  return (
    <CardListScreen
      search={search}
      connection={connection}
      mutations={mutations}
      ownerId={masterId}
      addCardEvent={{
        name: FLAMINGO_EVENT.addMasterCard,
        matches: (detail) => (detail as AddMasterCardDetail | undefined)?.masterId === masterId,
      }}
      bulkDeleteLog={{ scope: "[MasterCardsClient]", ownerKey: "masterId" }}
      sectionHeader={sectionHeader}
      importSheetTitle={deckName}
      renderImportForm={({ onImported, onCancel }) => (
        <MasterBatchImportForm
          masterId={masterId}
          deckName={deckName}
          onImported={onImported}
          onCancel={onCancel}
        />
      )}
    />
  );
}
