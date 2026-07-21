"use client";

import type { OperationVariables, TypedDocumentNode } from "@apollo/client";
import { useCallback, useMemo } from "react";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { makeMergeConnection } from "@/lib/pagination/make-merge-connection";
import {
  type UseConnectionPaginationResult,
  useConnectionPagination,
} from "@/lib/pagination/use-connection-pagination";

/**
 * Query variables every cards connection shares: the forward cursor and the
 * search filter. Each concrete connection's generated `…QueryVariables` widens
 * this with its owner-id field (`cardgroupId` / `masterCardgroupId`) and `first`.
 */
type CardsConnectionVariables = OperationVariables & {
  // `after` is an `ID` scalar in every cards connection — `string | number`.
  after?: string | number | null;
  search?: string | null;
};

/** The next-page flag + cursor the shared IO loop reads off `pageInfo`. */
interface PageInfoLike {
  hasNextPage: boolean;
  endCursor?: string | null;
}

/** The Relay connection slice the shared hook renders and advances. */
interface ConnectionShape<TEdge, TPageInfo> {
  edges: TEdge[];
  pageInfo: TPageInfo;
  totalCount: number;
}

/** Infers the edge type from a connection slice. */
type EdgeOf<TConn> = TConn extends { edges: readonly (infer TEdge)[] } ? TEdge : never;
/** Infers the pageInfo type from a connection slice. */
type PageInfoOf<TConn> = TConn extends { pageInfo: infer TPageInfo } ? TPageInfo : never;

/**
 * Static, per-entity binding for {@link useEntityCardsConnection}: the connection
 * document, the connection field key on the query result, the owner-scoped
 * default-vars factory, and the log scope. A module constant so the hook's
 * `useMemo` / `useCallback` memoization stays stable across renders.
 */
export interface EntityCardsConnectionConfig<
  TData,
  TVars extends CardsConnectionVariables,
  TConnKey extends keyof TData,
> {
  /** The typed connection query document, keyed for the underlying `useQuery`. */
  document: TypedDocumentNode<TData, TVars>;
  /** The connection field key on `TData`, e.g. `"cardsByCardgroupConnection"`. */
  connectionField: TConnKey;
  /**
   * Default connection vars factory, scoped by owner id (search: null, no
   * cursor). The same factory that seeds the RSC render and mutation cache
   * writes, so the cache key stays identical across all read sites.
   */
  defaultVars: (ownerId: string) => TVars;
  /** Log prefix for the structured `fetchMore failed` warn, e.g. `"[cards-client]"`. */
  logScope: string;
}

/**
 * Identity helper that supplies the contextual type for a config literal so
 * `connectionField` / `defaultVars` are checked against the document's inferred
 * `TData` / `TVars`, while keeping the config a stable module constant. Mirrors
 * `defineEntityCardMutationsConfig`.
 */
export function defineEntityCardsConnectionConfig<
  TData,
  TVars extends CardsConnectionVariables,
  TConnKey extends keyof TData,
>(
  config: EntityCardsConnectionConfig<TData, TVars, TConnKey>,
): EntityCardsConnectionConfig<TData, TVars, TConnKey> {
  return config;
}

export interface UseEntityCardsConnectionInput<TEdge, TPageInfo> {
  /** The owner id the connection is scoped to (cardgroup / master deck). */
  ownerId: string;
  searchQuery: string | null;
  initialEdges: TEdge[];
  initialPageInfo: TPageInfo;
  initialTotalCount: number;
  /**
   * Localized fallback banner for a fetchMore failure with no backend-mapped
   * message. The hook is not a component and cannot call `useTranslations`, so
   * the client passes the localized string in. The namespace tracks the LISTED
   * entity (cards), not the feature directory — see `.claude/rules/pagination.md`.
   */
  fetchMoreErrorMessage: string;
}

/**
 * Generic cards-connection pagination hook shared by the three per-entity
 * screens (cardgroup cards, admin master cards, catalog deck cards). Supplies
 * the connection-specific bits — the document, the `defaultVars`-based
 * variables, and the connection-field accessor + edges concat — from a static
 * `config`; the three pagination-rule invariants (IO in-flight guard, Effect
 * Event observer advance, split debounce-vs-immediate-reset) live in the
 * underlying `useConnectionPagination`. Thin per-entity wrappers bind the config
 * and translate their route id to `ownerId`.
 */
export function useEntityCardsConnection<
  TData,
  TVars extends CardsConnectionVariables,
  TConnKey extends keyof TData,
>(
  config: EntityCardsConnectionConfig<TData, TVars, TConnKey>,
  input: UseEntityCardsConnectionInput<EdgeOf<TData[TConnKey]>, PageInfoOf<TData[TConnKey]>>,
): UseConnectionPaginationResult<
  TData,
  EdgeOf<TData[TConnKey]>,
  PageInfoOf<TData[TConnKey]>,
  TVars
> {
  type TEdge = EdgeOf<TData[TConnKey]>;
  type TPageInfo = PageInfoOf<TData[TConnKey]>;

  const { document, connectionField, defaultVars, logScope } = config;
  const {
    ownerId,
    searchQuery,
    initialEdges,
    initialPageInfo,
    initialTotalCount,
    fetchMoreErrorMessage,
  } = input;

  // When searchQuery is null we use defaultVars verbatim so the cache key
  // matches the SSR seed exactly. For non-null searches we spread and override
  // `search`, keeping the owner id / `first` in sync with the default.
  const variables = useMemo<TVars>(
    () =>
      searchQuery === null
        ? defaultVars(ownerId)
        : ({ ...defaultVars(ownerId), search: searchQuery } as TVars),
    [defaultVars, ownerId, searchQuery],
  );

  const buildFetchMoreVariables = useCallback(
    (endCursor: string | null, search: string | null): TVars =>
      ({ ...defaultVars(ownerId), after: endCursor, search }) as TVars,
    [defaultVars, ownerId],
  );

  // Concatenate the next page's edges onto the cached connection under the
  // config's connection field. Memoized on `connectionField` so the reducer
  // identity stays stable across renders — `useConnectionPagination` feeds it
  // into a `useCallback` dependency array.
  const mergeConnection = useMemo(
    () => makeMergeConnection<TData>(connectionField),
    [connectionField],
  );

  return useConnectionPagination<TData, TVars, TEdge, TPageInfo & PageInfoLike>({
    document,
    variables,
    searchQuery,
    selectConnection: (data) =>
      data == null
        ? undefined
        : (data[connectionField] as unknown as ConnectionShape<TEdge, TPageInfo & PageInfoLike>),
    buildFetchMoreVariables,
    mergeConnection,
    initial: {
      edges: initialEdges,
      pageInfo: initialPageInfo as unknown as TPageInfo & PageInfoLike,
      totalCount: initialTotalCount,
    },
    resolveFetchMoreError: (err) => getBackendErrorBanner(err) ?? fetchMoreErrorMessage,
    logScope,
  });
}
