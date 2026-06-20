import { useCallback, useMemo } from "react";
import {
  CatalogMasterCardsConnectionDocument,
  type CatalogMasterCardsConnectionQuery,
  type CatalogMasterCardsConnectionQueryVariables,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import {
  type UseConnectionPaginationResult,
  useConnectionPagination,
} from "@/lib/pagination/use-connection-pagination";
import { catalogCardsDefaultVars } from "./queries";

type CatalogCardEdge = CatalogMasterCardsConnectionQuery["masterCardsConnection"]["edges"][number];
type CatalogCardPageInfo = CatalogMasterCardsConnectionQuery["masterCardsConnection"]["pageInfo"];

export interface UseCatalogCardsConnectionInput {
  masterCardgroupId: string;
  searchQuery: string | null;
  initialEdges: CatalogCardEdge[];
  initialPageInfo: CatalogCardPageInfo;
  initialTotalCount: number;
}

export type UseCatalogCardsConnectionResult = UseConnectionPaginationResult<
  CatalogMasterCardsConnectionQuery,
  CatalogCardEdge,
  CatalogCardPageInfo,
  CatalogMasterCardsConnectionQueryVariables
>;

// Concatenate the next page's edges onto the cached master-cards connection.
function mergeCatalogCardsConnection(
  prev: CatalogMasterCardsConnectionQuery,
  more: CatalogMasterCardsConnectionQuery,
): CatalogMasterCardsConnectionQuery {
  return {
    masterCardsConnection: {
      ...more.masterCardsConnection,
      edges: [...prev.masterCardsConnection.edges, ...more.masterCardsConnection.edges],
    },
  };
}

// Catalog deck-detail card pagination — a thin wrapper over the generic
// useConnectionPagination hook, mirroring `useCardsConnection`. Supplies the
// catalog-specific bits: the document, the catalogCardsDefaultVars-based
// variables, and the data?.masterCardsConnection accessor + edges concat. The
// three pagination-rule invariants (in-flight guard, Effect Event observer
// advance, split debounce-vs-immediate-reset) live in the generic hook.
export function useCatalogCardsConnection(
  input: UseCatalogCardsConnectionInput,
): UseCatalogCardsConnectionResult {
  const { masterCardgroupId, searchQuery, initialEdges, initialPageInfo, initialTotalCount } =
    input;

  // When searchQuery is null we use catalogCardsDefaultVars verbatim so the
  // cache key matches the SSR seed exactly. For non-null searches we spread and
  // override `search`, keeping `masterCardgroupId`/`first` in sync with the default.
  const variables = useMemo<CatalogMasterCardsConnectionQueryVariables>(
    () =>
      searchQuery === null
        ? catalogCardsDefaultVars(masterCardgroupId)
        : { ...catalogCardsDefaultVars(masterCardgroupId), search: searchQuery },
    [masterCardgroupId, searchQuery],
  );

  const buildFetchMoreVariables = useCallback(
    (
      endCursor: string | null,
      search: string | null,
    ): CatalogMasterCardsConnectionQueryVariables => ({
      ...catalogCardsDefaultVars(masterCardgroupId),
      after: endCursor,
      search,
    }),
    [masterCardgroupId],
  );

  return useConnectionPagination<
    CatalogMasterCardsConnectionQuery,
    CatalogMasterCardsConnectionQueryVariables,
    CatalogCardEdge,
    CatalogCardPageInfo
  >({
    document: CatalogMasterCardsConnectionDocument,
    variables,
    searchQuery,
    selectConnection: (data) => data?.masterCardsConnection,
    buildFetchMoreVariables,
    mergeConnection: mergeCatalogCardsConnection,
    initial: { edges: initialEdges, pageInfo: initialPageInfo, totalCount: initialTotalCount },
    resolveFetchMoreError: (err) =>
      getBackendErrorBanner(err) ?? "Could not load more cards. Please try again.",
    logScope: "[catalog-deck]",
  });
}
