"use client";

import { NetworkStatus } from "@apollo/client";
import { useTranslations } from "next-intl";
import { useMemo } from "react";
import { ConnectionListFooter } from "@/components/layout/connection-list-footer";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { SearchInput } from "@/components/search/search-input";
import { SearchTakeoverBar } from "@/components/search/search-takeover-bar";
import {
  MasterCatalogDocument,
  type MasterCatalogQuery,
  type MasterCatalogQueryVariables,
} from "@/generated/graphql";
import { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";
import { useSeedConnectionCache } from "@/lib/pagination/use-seed-connection-cache";
import { CatalogListItem } from "./catalog-list-item";
import { CATALOG_DEFAULT_VARS, CATALOG_INITIAL, mergeCatalogConnection } from "./queries";

type Connection = MasterCatalogQuery["masterCatalog"];
type CatalogEdge = Connection["edges"][number];
type CatalogPageInfo = Connection["pageInfo"];

interface CatalogClientProps {
  initialConnection: Connection | null;
}

/**
 * Client component for the /catalog list.
 *
 * Wires:
 *  - SSR seed: writes initialConnection into the cache once synchronously during
 *    render (before useQuery runs) via apollo.writeQuery so useQuery (cache-first)
 *    renders immediately without a network round-trip. CATALOG_DEFAULT_VARS keeps
 *    the cache key identical to the SSR seed and the client useQuery — any
 *    mismatch silently splits the cache.
 *  - Debounced search (300ms; search.input → searchQuery), passed as the `search`
 *    variable on MasterCatalogQuery.
 *  - Infinite scroll via useConnectionPagination, which owns the IntersectionObserver
 *    and the in-flight useRef<boolean> guard (per docs/pagination/intersection-observer-in-flight-guard.md).
 *  - fetchMoreError halt gate — the observer short-circuits while an error banner
 *    is showing; the user must click Retry to resume.
 */
export default function CatalogClient({ initialConnection }: CatalogClientProps) {
  const t = useTranslations("Catalog");
  const tCommon = useTranslations("Common");

  const search = useHeaderTakeoverSearch();
  const searchQuery = search.query;

  // Seed the SSR connection into the cache synchronously before useQuery runs.
  // CATALOG_DEFAULT_VARS keeps the cache key identical to the SSR seed and the
  // client useQuery — any mismatch silently splits the cache.
  useSeedConnectionCache({
    document: MasterCatalogDocument,
    variables: CATALOG_DEFAULT_VARS,
    data: { masterCatalog: initialConnection },
    warnScope: "catalog-client",
  });

  // When searchQuery is null we use CATALOG_DEFAULT_VARS verbatim so the cache key
  // matches the SSR seed exactly. For non-null searches we spread and override
  // `search`, keeping `first` in sync with the default. Memoized so the hook's
  // useQuery does not re-subscribe on unrelated re-renders.
  const queryVariables = useMemo(
    () =>
      searchQuery === null
        ? CATALOG_DEFAULT_VARS
        : { ...CATALOG_DEFAULT_VARS, search: searchQuery },
    [searchQuery],
  );

  const {
    edges,
    pageInfo,
    totalCount,
    loading,
    networkStatus,
    fetchingMore,
    fetchMoreError,
    retryFetchMore,
    sentinelRef,
  } = useConnectionPagination<
    MasterCatalogQuery,
    MasterCatalogQueryVariables,
    CatalogEdge,
    CatalogPageInfo
  >({
    document: MasterCatalogDocument,
    variables: queryVariables,
    searchQuery: searchQuery,
    selectConnection: (data) => data?.masterCatalog,
    buildFetchMoreVariables: (after, searchValue) => ({
      ...CATALOG_DEFAULT_VARS,
      after,
      search: searchValue,
    }),
    mergeConnection: mergeCatalogConnection,
    initial: CATALOG_INITIAL,
    // Catalog deliberately shows a single generic banner for every fetchMore
    // failure (no backend-message surfacing), matching the inline original.
    resolveFetchMoreError: () => t("fetchMoreError"),
    logScope: "[catalog]",
  });
  const hasNextPage = pageInfo.hasNextPage;

  const initialLoading = loading && edges.length === 0 && networkStatus !== NetworkStatus.fetchMore;
  const hasSearch = searchQuery !== null && searchQuery !== "";

  return (
    <>
      <SearchTakeoverBar
        open={search.searchOpen}
        value={search.input}
        onChange={search.setInput}
        onClear={search.clear}
        onClose={search.closeSearch}
        placeholder={t("searchPlaceholder")}
        ariaLabel={t("searchAriaLabel")}
      />
      <ListingPageShell
        title={t("title")}
        count={totalCount}
        countLabel={tCommon("totalCount", { count: totalCount })}
        toolbar={
          <div className="mb-2 hidden md:block">
            <SearchInput
              placeholder={t("searchPlaceholder")}
              value={search.input}
              onChange={(e) => search.setInput(e.target.value)}
              aria-label={t("searchAriaLabel")}
              data-testid="catalog-search"
            />
          </div>
        }
      >
        {initialLoading && (
          <p className="text-sm text-muted-foreground" data-testid="catalog-loading">
            {tCommon("loading")}
          </p>
        )}

        {!initialLoading && edges.length === 0 && !hasSearch && (
          <p className="text-sm text-muted-foreground" data-testid="catalog-empty">
            {t("noDecks")}
          </p>
        )}

        {!initialLoading && edges.length === 0 && hasSearch && (
          <p className="text-sm text-muted-foreground" data-testid="catalog-empty-search">
            {t("noMatch", { query: searchQuery })}
          </p>
        )}

        {edges.length > 0 && (
          <ul className="space-y-2" data-testid="catalog-list">
            {edges.map((edge) => (
              <CatalogListItem key={edge.cursor} node={edge.node} />
            ))}
          </ul>
        )}

        <ConnectionListFooter
          sentinelRef={sentinelRef}
          fetchMoreError={fetchMoreError}
          onRetry={retryFetchMore}
          fetchingMore={fetchingMore}
          hasNextPage={hasNextPage}
          retryLabel={tCommon("retry")}
          loadingMoreLabel={t("loadingMore")}
          testIdPrefix="catalog"
        />
      </ListingPageShell>
    </>
  );
}
