import { useCallback, useMemo } from "react";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import {
  type UseConnectionPaginationResult,
  useConnectionPagination,
} from "@/lib/pagination/use-connection-pagination";
import { cardsDefaultVars } from "./queries";

type CardEdge = CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"]["edges"][number];
type CardConnectionPageInfo =
  CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"]["pageInfo"];

export interface UseCardsConnectionInput {
  cardgroupId: string;
  searchQuery: string | null;
  initialEdges: CardEdge[];
  initialPageInfo: CardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Localized fallback banner for a fetchMore failure with no backend-mapped
   * message. The hook is not a component and cannot call `useTranslations`, so
   * the client passes the localized string in (mirrors `useMasterCardsConnection`).
   */
  fetchMoreErrorMessage: string;
}

export type UseCardsConnectionResult = UseConnectionPaginationResult<
  CardsByCardgroupConnectionQuery,
  CardEdge,
  CardConnectionPageInfo,
  CardsByCardgroupConnectionQueryVariables
>;

// Concatenate the next page's edges onto the cached cards connection.
function mergeCardsConnection(
  prev: CardsByCardgroupConnectionQuery,
  more: CardsByCardgroupConnectionQuery,
): CardsByCardgroupConnectionQuery {
  return {
    cardsByCardgroupConnection: {
      ...more.cardsByCardgroupConnection,
      edges: [...prev.cardsByCardgroupConnection.edges, ...more.cardsByCardgroupConnection.edges],
    },
  };
}

// Cards-by-cardgroup pagination — a thin wrapper over the generic
// useConnectionPagination hook. Supplies the three cards-specific bits: the
// document, the cardsDefaultVars-based variables, and the
// data?.cardsByCardgroupConnection accessor + edges concat. The three
// pagination-rule invariants (in-flight guard, Effect Event observer advance,
// split debounce-vs-immediate-reset) live in the generic hook.
export function useCardsConnection(input: UseCardsConnectionInput): UseCardsConnectionResult {
  const {
    cardgroupId,
    searchQuery,
    initialEdges,
    initialPageInfo,
    initialTotalCount,
    fetchMoreErrorMessage,
  } = input;

  // When searchQuery is null we use cardsDefaultVars verbatim so the cache key
  // matches the SSR seed exactly. For non-null searches we spread and override
  // `search`, keeping `cardgroupId`/`first` in sync with the default.
  const variables = useMemo<CardsByCardgroupConnectionQueryVariables>(
    () =>
      searchQuery === null
        ? cardsDefaultVars(cardgroupId)
        : { ...cardsDefaultVars(cardgroupId), search: searchQuery },
    [cardgroupId, searchQuery],
  );

  const buildFetchMoreVariables = useCallback(
    (
      endCursor: string | null,
      search: string | null,
    ): CardsByCardgroupConnectionQueryVariables => ({
      ...cardsDefaultVars(cardgroupId),
      after: endCursor,
      search,
    }),
    [cardgroupId],
  );

  return useConnectionPagination<
    CardsByCardgroupConnectionQuery,
    CardsByCardgroupConnectionQueryVariables,
    CardEdge,
    CardConnectionPageInfo
  >({
    document: CardsByCardgroupConnectionDocument,
    variables,
    searchQuery,
    selectConnection: (data) => data?.cardsByCardgroupConnection,
    buildFetchMoreVariables,
    mergeConnection: mergeCardsConnection,
    initial: { edges: initialEdges, pageInfo: initialPageInfo, totalCount: initialTotalCount },
    resolveFetchMoreError: (err) => getBackendErrorBanner(err) ?? fetchMoreErrorMessage,
    logScope: "[cards-client]",
  });
}
