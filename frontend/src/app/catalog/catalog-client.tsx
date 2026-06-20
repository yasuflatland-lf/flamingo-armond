"use client";

import { NetworkStatus } from "@apollo/client";
import { useApolloClient } from "@apollo/client/react";
import { useTranslations } from "next-intl";
import { useMemo, useRef } from "react";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { SearchTakeoverBar } from "@/components/search/search-takeover-bar";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import {
  MasterCatalogDocument,
  type MasterCatalogQuery,
  type MasterCatalogQueryVariables,
} from "@/generated/graphql";
import { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import { EMPTY_PAGE_INFO } from "@/lib/pagination/empty-page-info";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";
import { CatalogListItem } from "./catalog-list-item";
import { CATALOG_DEFAULT_VARS } from "./queries";

type Connection = MasterCatalogQuery["masterCatalog"];
type CatalogEdge = Connection["edges"][number];
type CatalogPageInfo = Connection["pageInfo"];

interface CatalogClientProps {
  initialConnection: Connection | null;
}

// Render fallback for useConnectionPagination. The client seeds the cache
// synchronously before useQuery runs, so this is never read on the happy path;
// it keeps the empty-edges shape the inline implementation used (`?? []`).
const CATALOG_INITIAL = {
  edges: [] as CatalogEdge[],
  pageInfo: EMPTY_PAGE_INFO,
  totalCount: 0,
};

// Concatenate the next page's edges onto the cached catalog connection.
function mergeCatalogConnection(
  prev: MasterCatalogQuery,
  more: MasterCatalogQuery,
): MasterCatalogQuery {
  return {
    masterCatalog: {
      ...more.masterCatalog,
      edges: [...prev.masterCatalog.edges, ...more.masterCatalog.edges],
    },
  };
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
  const apollo = useApolloClient();
  const t = useTranslations("Catalog");
  const tCommon = useTranslations("Common");

  const search = useHeaderTakeoverSearch();
  const searchQuery = search.query;

  // Strict Mode double-mount safety: only write the SSR seed into the cache once.
  const seededRef = useRef(false);

  // Seed the cache synchronously during render (before useQuery runs) with the
  // SSR initialConnection so the first useQuery pass (cache-first) finds the data
  // already in the cache and renders without a network round-trip. Doing this in
  // a useEffect would create a window between first paint and the post-render
  // write where useQuery sees an empty cache. The seededRef guard is synchronous,
  // so it survives Strict Mode's double-invoke without a second write.
  if (!seededRef.current && initialConnection === null) {
    console.warn(
      "[catalog-client] initialConnection is null — SSR seed skipped; useQuery will fetch fresh",
    );
  }
  if (!seededRef.current && initialConnection != null) {
    seededRef.current = true;
    apollo.writeQuery({
      query: MasterCatalogDocument,
      variables: CATALOG_DEFAULT_VARS,
      data: { masterCatalog: initialConnection },
    });
  }

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
        toolbar={
          <div className="mb-2 hidden md:block">
            <input
              type="search"
              placeholder={t("searchPlaceholder")}
              value={search.input}
              onChange={(e) => search.setInput(e.target.value)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
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

        <div ref={sentinelRef} aria-hidden="true" data-testid="catalog-sentinel" />

        {fetchMoreError && (
          <ErrorBanner
            className="mt-3 flex flex-col items-center gap-2"
            data-testid="catalog-fetch-more-error"
          >
            <span>{fetchMoreError}</span>
            <Button type="button" variant="outline" size="sm" onClick={retryFetchMore}>
              {tCommon("retry")}
            </Button>
          </ErrorBanner>
        )}

        {!fetchMoreError && fetchingMore && hasNextPage && (
          <p
            className="mt-3 text-center text-xs text-muted-foreground"
            data-testid="catalog-loading-more"
          >
            {t("loadingMore")}
          </p>
        )}
      </ListingPageShell>
    </>
  );
}
