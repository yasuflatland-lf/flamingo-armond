import { type ErrorLike, NetworkStatus } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import type { RefObject } from "react";
import { useCallback, useEffect, useEffectEvent, useMemo, useRef, useState } from "react";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import type { FetchNextPageInput } from "@/lib/pagination/types";
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
}

export interface UseCardsConnectionResult {
  edges: CardEdge[];
  pageInfo: CardConnectionPageInfo;
  totalCount: number;
  loading: boolean;
  networkStatus: NetworkStatus;
  fetchingMore: boolean;
  fetchMoreError: string | null;
  retryFetchMore: () => void;
  sentinelRef: RefObject<HTMLDivElement | null>;
  queryVariables: CardsByCardgroupConnectionQueryVariables;
  queryError: ErrorLike | undefined;
}

// Apollo connection + fetchMore + IntersectionObserver hook for the
// cards-by-cardgroup paginated query. Preserves three invariants from
// .claude/rules/pagination.md: the IO in-flight guard (useRef<boolean>), the
// React 19.2 useEffectEvent observer advance path, and the split
// debounce-vs-immediate-reset effects driven by searchQuery changes.
export function useCardsConnection(input: UseCardsConnectionInput): UseCardsConnectionResult {
  const { cardgroupId, searchQuery, initialEdges, initialPageInfo, initialTotalCount } = input;

  const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);

  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // In-flight guard MUST be useRef<boolean>, not useState — see
  // docs/pagination/intersection-observer-in-flight-guard.md.
  const fetchingRef = useRef(false);

  // When the active search query changes, any in-flight fetchMore from the
  // previous search holds a stale cursor. Reset the IO guard and error state
  // immediately so the new query starts from a clean slate.
  // See docs/pagination/intersection-observer-in-flight-guard.md.
  // biome-ignore lint/correctness/useExhaustiveDependencies: searchQuery is an intentional trigger dependency; it is not referenced in the body because the effect resets derived IO state, not searchQuery itself.
  useEffect(() => {
    fetchingRef.current = false;
    setFetchMoreError(null);
  }, [searchQuery]);

  // When searchQuery is null we use cardsDefaultVars verbatim so the cache key
  // matches the SSR seed exactly. For non-null searches we spread and override
  // `search`, keeping `cardgroupId`/`first` in sync with the default.
  const queryVariables: CardsByCardgroupConnectionQueryVariables = useMemo(
    () =>
      searchQuery === null
        ? cardsDefaultVars(cardgroupId)
        : { ...cardsDefaultVars(cardgroupId), search: searchQuery },
    [cardgroupId, searchQuery],
  );

  const {
    data,
    fetchMore,
    loading,
    networkStatus,
    error: queryError,
  } = useQuery(CardsByCardgroupConnectionDocument, {
    variables: queryVariables,
    fetchPolicy: "cache-first",
    notifyOnNetworkStatusChange: true,
  });

  const connection = data?.cardsByCardgroupConnection;
  const edges = connection?.edges ?? initialEdges;
  const pageInfo = connection?.pageInfo ?? initialPageInfo;
  const totalCount = connection?.totalCount ?? initialTotalCount;

  const fetchNextPage = useCallback(
    ({ hasNextPage, endCursor, searchQuery }: FetchNextPageInput) => {
      if (fetchingRef.current) return;
      if (!hasNextPage) return;

      fetchingRef.current = true;
      fetchMore({
        variables: {
          ...cardsDefaultVars(cardgroupId),
          after: endCursor,
          search: searchQuery,
        },
        updateQuery: (prev, { fetchMoreResult }) => {
          if (!fetchMoreResult) return prev;
          return {
            cardsByCardgroupConnection: {
              ...fetchMoreResult.cardsByCardgroupConnection,
              edges: [
                ...prev.cardsByCardgroupConnection.edges,
                ...fetchMoreResult.cardsByCardgroupConnection.edges,
              ],
            },
          };
        },
      })
        .then(() => {
          // Clear any previous fetchMore error on success so the observer can resume.
          setFetchMoreError(null);
        })
        .catch((err) => {
          // Structured warn for operator triage: name + request context only.
          // err.message is omitted — backend messages may carry user-authored content.
          // See docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
          console.warn("[cards-client] fetchMore failed", {
            name: err instanceof Error ? err.name : "unknown",
            searchQuery,
            endCursor,
          });
          const banner =
            getBackendErrorBanner(err) ?? "Could not load more cards. Please try again.";
          setFetchMoreError(banner);
        })
        .finally(() => {
          fetchingRef.current = false;
        });
    },
    [cardgroupId, fetchMore],
  );

  const requestNextPageFromObserver = useEffectEvent(() => {
    fetchNextPage({
      hasNextPage: pageInfo.hasNextPage,
      endCursor: pageInfo.endCursor ?? null,
      searchQuery,
    });
  });

  useEffect(() => {
    if (!pageInfo.hasNextPage) return;
    // Stop the observer loop while a previous fetch failed; user must click Retry to resume.
    if (fetchMoreError != null) return;
    const node = sentinelRef.current;
    if (!node) return;

    const observer = new IntersectionObserver((entries) => {
      const entry = entries[0];
      if (!entry?.isIntersecting) return;
      if (fetchingRef.current) return;
      requestNextPageFromObserver();
    });

    observer.observe(node);
    return () => observer.disconnect();
  }, [pageInfo.hasNextPage, fetchMoreError]);

  const retryFetchMore = useCallback(() => {
    setFetchMoreError(null);
    fetchNextPage({
      hasNextPage: pageInfo.hasNextPage,
      endCursor: pageInfo.endCursor ?? null,
      searchQuery,
    });
  }, [fetchNextPage, pageInfo.endCursor, pageInfo.hasNextPage, searchQuery]);

  const fetchingMore = networkStatus === NetworkStatus.fetchMore || (loading && edges.length > 0);

  return {
    edges,
    pageInfo,
    totalCount,
    loading,
    networkStatus,
    fetchingMore,
    fetchMoreError,
    retryFetchMore,
    sentinelRef,
    queryVariables,
    queryError,
  };
}
