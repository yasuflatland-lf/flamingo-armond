import { type ErrorLike, NetworkStatus } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import type { RefObject } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
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
// cursor/search/hasNextPage ref triplet that keeps requestNextPage's identity
// stable across page advances, and the split debounce-vs-immediate-reset
// effects driven by searchQuery changes.
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

  // Mirror cursor-related page state into refs so requestNextPage can read them
  // without being listed as a dep. This prevents the IO observer effect from
  // disconnecting/reconnecting every time a page loads (which updates endCursor).
  // See docs/pagination/stabilise-request-next-page-ref-triplet.md.
  const endCursorRef = useRef(pageInfo.endCursor);
  const hasNextPageRef = useRef(pageInfo.hasNextPage);
  const searchQueryRef = useRef(searchQuery);
  useEffect(() => {
    endCursorRef.current = pageInfo.endCursor;
  }, [pageInfo.endCursor]);
  useEffect(() => {
    hasNextPageRef.current = pageInfo.hasNextPage;
  }, [pageInfo.hasNextPage]);
  useEffect(() => {
    searchQueryRef.current = searchQuery;
  }, [searchQuery]);

  const requestNextPage = useCallback(() => {
    if (fetchingRef.current) return;
    if (!hasNextPageRef.current) return;

    fetchingRef.current = true;
    fetchMore({
      variables: {
        ...cardsDefaultVars(cardgroupId),
        after: endCursorRef.current,
        search: searchQueryRef.current,
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
          searchQuery: searchQueryRef.current ?? null,
          endCursor: endCursorRef.current ?? null,
        });
        const banner = getBackendErrorBanner(err) ?? "Could not load more cards. Please try again.";
        setFetchMoreError(banner);
      })
      .finally(() => {
        fetchingRef.current = false;
      });
  }, [cardgroupId, fetchMore]);

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
      if (!hasNextPageRef.current) return;
      requestNextPage();
    });

    observer.observe(node);
    return () => observer.disconnect();
  }, [pageInfo.hasNextPage, fetchMoreError, requestNextPage]);

  const retryFetchMore = useCallback(() => {
    setFetchMoreError(null);
    requestNextPage();
  }, [requestNextPage]);

  const fetchingMore = networkStatus === NetworkStatus.fetchMore;

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
