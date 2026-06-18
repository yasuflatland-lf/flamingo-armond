import { useCallback, useMemo } from "react";
import {
  AdminMasterCardsConnectionDocument,
  type AdminMasterCardsConnectionQuery,
  type AdminMasterCardsConnectionQueryVariables,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import {
  type UseConnectionPaginationResult,
  useConnectionPagination,
} from "@/lib/pagination/use-connection-pagination";
import { masterCardsDefaultVars } from "./queries";

type MasterCardEdge =
  AdminMasterCardsConnectionQuery["adminMasterCardsConnection"]["edges"][number];
type MasterCardConnectionPageInfo =
  AdminMasterCardsConnectionQuery["adminMasterCardsConnection"]["pageInfo"];

export interface UseMasterCardsConnectionInput {
  masterId: string;
  searchQuery: string | null;
  initialEdges: MasterCardEdge[];
  initialPageInfo: MasterCardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Localized banner copy shown when a load-more page fails with no backend
   * banner of its own. The hook has no `useTranslations`, so the caller passes
   * the resolved `t("fetchMoreFailed")` string in (mirrors admin-masters-client).
   */
  fetchMoreErrorMessage: string;
}

export type UseMasterCardsConnectionResult = UseConnectionPaginationResult<
  AdminMasterCardsConnectionQuery,
  MasterCardEdge,
  MasterCardConnectionPageInfo,
  AdminMasterCardsConnectionQueryVariables
>;

function mergeMasterCardsConnection(
  prev: AdminMasterCardsConnectionQuery,
  more: AdminMasterCardsConnectionQuery,
): AdminMasterCardsConnectionQuery {
  return {
    adminMasterCardsConnection: {
      ...more.adminMasterCardsConnection,
      edges: [...prev.adminMasterCardsConnection.edges, ...more.adminMasterCardsConnection.edges],
    },
  };
}

export function useMasterCardsConnection(
  input: UseMasterCardsConnectionInput,
): UseMasterCardsConnectionResult {
  const {
    masterId,
    searchQuery,
    initialEdges,
    initialPageInfo,
    initialTotalCount,
    fetchMoreErrorMessage,
  } = input;

  const variables = useMemo<AdminMasterCardsConnectionQueryVariables>(
    () =>
      searchQuery === null
        ? masterCardsDefaultVars(masterId)
        : { ...masterCardsDefaultVars(masterId), search: searchQuery },
    [masterId, searchQuery],
  );

  const buildFetchMoreVariables = useCallback(
    (
      endCursor: string | null,
      search: string | null,
    ): AdminMasterCardsConnectionQueryVariables => ({
      ...masterCardsDefaultVars(masterId),
      after: endCursor,
      search,
    }),
    [masterId],
  );

  return useConnectionPagination<
    AdminMasterCardsConnectionQuery,
    AdminMasterCardsConnectionQueryVariables,
    MasterCardEdge,
    MasterCardConnectionPageInfo
  >({
    document: AdminMasterCardsConnectionDocument,
    variables,
    searchQuery,
    selectConnection: (data) => data?.adminMasterCardsConnection,
    buildFetchMoreVariables,
    mergeConnection: mergeMasterCardsConnection,
    initial: { edges: initialEdges, pageInfo: initialPageInfo, totalCount: initialTotalCount },
    resolveFetchMoreError: (err) => getBackendErrorBanner(err) ?? fetchMoreErrorMessage,
    logScope: "[master-cards-client]",
  });
}
