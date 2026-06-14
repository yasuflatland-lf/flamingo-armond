import {
  type ErrorLike,
  NetworkStatus,
  type OperationVariables,
  type TypedDocumentNode,
} from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import type { RefObject } from "react";
import { useCallback, useEffect, useEffectEvent, useRef, useState } from "react";
import type { FetchNextPageInput } from "@/lib/pagination/types";

/** Minimal `pageInfo` shape the IO loop reads: the next-page flag and the cursor. */
interface PageInfoLike {
  hasNextPage: boolean;
  endCursor?: string | null;
}

/** The Relay connection slice the hook renders and advances. */
interface ConnectionShape<TEdge, TPageInfo> {
  edges: TEdge[];
  pageInfo: TPageInfo;
  totalCount: number;
}

export interface UseConnectionPaginationInput<
  TData,
  TVars extends OperationVariables,
  TEdge,
  TPageInfo extends PageInfoLike,
> {
  /** The typed connection query document. */
  document: TypedDocumentNode<TData, TVars>;
  /**
   * The variables for the active query. The caller weaves the debounced
   * `searchQuery` into these; the hook does not mutate them. Memoize at the
   * call site so the underlying `useQuery` does not re-subscribe each render.
   */
  variables: TVars;
  /**
   * The active debounced search value (or `null`). Used only as the
   * immediate-reset trigger — when it changes, the in-flight guard and
   * fetch-more error are cleared so a stale-cursor fetch from the previous
   * search cannot complete against the new query.
   */
  searchQuery: string | null;
  /** Reads the connection slice out of the query result (document-specific field). */
  selectConnection: (data: TData | undefined) => ConnectionShape<TEdge, TPageInfo> | undefined;
  /** Builds the `fetchMore` variables for the next page given the cursor + search. */
  buildFetchMoreVariables: (endCursor: string | null, searchQuery: string | null) => TVars;
  /** Concatenates the next page's edges onto the previous result (document-specific field). */
  mergeConnection: (prev: TData, more: TData) => TData;
  /**
   * Render fallback used until the query resolves. For SSR-seeded screens this
   * is never read (the cache seed makes the first `useQuery` pass synchronous);
   * for prop-seeded screens it is the initial render value.
   */
  initial: ConnectionShape<TEdge, TPageInfo>;
  /** Maps a failed `fetchMore` error to the user-facing banner string. */
  resolveFetchMoreError: (err: unknown) => string;
  /** Log prefix for the structured `fetchMore failed` warn, e.g. `"[cardgroups]"`. */
  logScope: string;
}

export interface UseConnectionPaginationResult<
  TData,
  TEdge,
  TPageInfo,
  TVars extends OperationVariables,
> {
  edges: TEdge[];
  pageInfo: TPageInfo;
  totalCount: number;
  loading: boolean;
  networkStatus: NetworkStatus;
  fetchingMore: boolean;
  fetchMoreError: string | null;
  retryFetchMore: () => void;
  sentinelRef: RefObject<HTMLDivElement | null>;
  queryVariables: TVars;
  queryError: ErrorLike | undefined;
  /**
   * The underlying `useQuery` refetch. Exposed so a screen that renders a
   * query-error banner can offer a Retry, and so a mutation-conflict reload
   * (admin users) can re-issue the list query. The SSR-seeded screens
   * (cards / cardgroups / catalog) do not consume it.
   */
  refetch: useQuery.Result<TData, TVars>["refetch"];
}

// Generic Apollo connection + fetchMore + IntersectionObserver hook for
// cursor-paginated lists. Generalised from the cards-by-cardgroup hook;
// the only document-specific bits (the query, the variables factory, the
// connection accessor + edges concat) are injected by the caller. Preserves
// three invariants from .claude/rules/pagination.md: the IO in-flight guard
// (useRef<boolean>), the React 19.2 useEffectEvent observer advance path, and
// the immediate-reset effect driven by searchQuery changes.
export function useConnectionPagination<
  TData,
  TVars extends OperationVariables,
  TEdge,
  TPageInfo extends PageInfoLike,
>(
  input: UseConnectionPaginationInput<TData, TVars, TEdge, TPageInfo>,
): UseConnectionPaginationResult<TData, TEdge, TPageInfo, TVars> {
  const {
    document,
    variables,
    searchQuery,
    selectConnection,
    buildFetchMoreVariables,
    mergeConnection,
    initial,
    resolveFetchMoreError,
    logScope,
  } = input;

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

  const {
    data,
    fetchMore,
    loading,
    networkStatus,
    error: queryError,
    refetch,
  } = useQuery(document, {
    variables,
    fetchPolicy: "cache-first",
    notifyOnNetworkStatusChange: true,
  });

  const connection = selectConnection(data);
  const edges = connection?.edges ?? initial.edges;
  const pageInfo = connection?.pageInfo ?? initial.pageInfo;
  const totalCount = connection?.totalCount ?? initial.totalCount;

  const fetchNextPage = useCallback(
    ({ hasNextPage, endCursor, searchQuery }: FetchNextPageInput) => {
      if (fetchingRef.current || !hasNextPage) return;

      fetchingRef.current = true;
      fetchMore({
        variables: buildFetchMoreVariables(endCursor, searchQuery),
        updateQuery: (prev, { fetchMoreResult }) => {
          if (!fetchMoreResult) return prev;
          return mergeConnection(prev, fetchMoreResult);
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
          console.warn(`${logScope} fetchMore failed`, {
            name: err instanceof Error ? err.name : "unknown",
            searchQuery,
            endCursor,
          });
          setFetchMoreError(resolveFetchMoreError(err));
        })
        .finally(() => {
          fetchingRef.current = false;
        });
    },
    [fetchMore, buildFetchMoreVariables, mergeConnection, resolveFetchMoreError, logScope],
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
      if (!entries[0]?.isIntersecting || fetchingRef.current) return;
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
    queryVariables: variables,
    queryError,
    refetch,
  };
}
