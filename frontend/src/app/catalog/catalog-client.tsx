"use client";

import { NetworkStatus } from "@apollo/client";
import { useApolloClient, useQuery } from "@apollo/client/react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useEffectEvent, useRef, useState } from "react";
import { toast } from "sonner";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Button } from "@/components/ui/button";
import { MasterCatalogDocument, type MasterCatalogQuery } from "@/generated/graphql";
import type { FetchNextPageInput } from "@/lib/pagination/types";
import { CatalogCard } from "./catalog-card";
import { CATALOG_DEFAULT_VARS } from "./queries";
import { useImportMaster } from "./use-import-master";

type Connection = MasterCatalogQuery["masterCatalog"];

interface CatalogClientProps {
  initialConnection: Connection | null;
}

/**
 * Client component for the /catalog gallery.
 *
 * Wires:
 *  - SSR seed: writes initialConnection into the cache once synchronously during
 *    render (before useQuery runs) via apollo.writeQuery so useQuery (cache-first)
 *    renders immediately without a network round-trip. CATALOG_DEFAULT_VARS keeps
 *    the cache key identical to the SSR seed and the client useQuery — any
 *    mismatch silently splits the cache.
 *  - Debounced search (300ms; searchInput → searchQuery), passed as the `search`
 *    variable on MasterCatalogQuery.
 *  - Infinite scroll via IntersectionObserver, with an in-flight guard via
 *    useRef<boolean> (per docs/pagination/intersection-observer-in-flight-guard.md).
 *  - fetchMoreError halt gate — the observer short-circuits while an error banner
 *    is showing; the user must click Retry to resume.
 *  - Per-item Import via useImportMaster, surfacing success / not-found / auth /
 *    rejection. On success the imported deck is prepended to the /cardgroups
 *    connection cache inside the hook.
 */
export default function CatalogClient({ initialConnection }: CatalogClientProps) {
  const apollo = useApolloClient();
  const t = useTranslations("Catalog");
  const tCommon = useTranslations("Common");

  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState<string | null>(null);
  const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);

  // Import state. `importingId` serializes imports to one at a time;
  // `importedIds` drives the per-card "Imported" affordance.
  const { importMasterCardgroup } = useImportMaster();
  const [importingId, setImportingId] = useState<string | null>(null);
  const [importedIds, setImportedIds] = useState<ReadonlySet<string>>(() => new Set());
  const [importError, setImportError] = useState<string | null>(null);
  const [importAuthError, setImportAuthError] = useState<"unauthenticated" | "forbidden" | null>(
    null,
  );

  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // In-flight guard MUST be useRef<boolean>, not useState — see
  // docs/pagination/intersection-observer-in-flight-guard.md.
  const fetchingRef = useRef(false);
  // Strict Mode double-mount safety: only write the SSR seed into the cache once.
  const seededRef = useRef(false);

  // Debounce: update searchQuery 300ms after the last keystroke.
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearchQuery(searchInput.trim() || null);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // When the active search query changes, any in-flight fetchMore from the
  // previous search holds a stale cursor. Reset the IO guard and error state
  // immediately so the new query starts from a clean slate.
  // biome-ignore lint/correctness/useExhaustiveDependencies: searchQuery is an intentional trigger dependency; it is not referenced in the body because the effect resets derived IO state, not searchQuery itself.
  useEffect(() => {
    fetchingRef.current = false;
    setFetchMoreError(null);
  }, [searchQuery]);

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
  // `search`, keeping `first` in sync with the default.
  const queryVariables =
    searchQuery === null ? CATALOG_DEFAULT_VARS : { ...CATALOG_DEFAULT_VARS, search: searchQuery };

  const { data, fetchMore, loading, networkStatus } = useQuery(MasterCatalogDocument, {
    variables: queryVariables,
    fetchPolicy: "cache-first",
    notifyOnNetworkStatusChange: true,
  });

  const connection = data?.masterCatalog;
  const edges = connection?.edges ?? [];
  const hasNextPage = connection?.pageInfo.hasNextPage ?? false;
  const endCursor = connection?.pageInfo.endCursor ?? null;

  const fetchNextPage = useCallback(
    ({ hasNextPage, endCursor, searchQuery }: FetchNextPageInput) => {
      if (fetchingRef.current || !hasNextPage) return;

      fetchingRef.current = true;
      fetchMore({
        variables: { ...CATALOG_DEFAULT_VARS, after: endCursor, search: searchQuery },
        updateQuery: (prev, { fetchMoreResult }) => {
          if (!fetchMoreResult) return prev;
          return {
            masterCatalog: {
              ...fetchMoreResult.masterCatalog,
              edges: [...prev.masterCatalog.edges, ...fetchMoreResult.masterCatalog.edges],
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
          console.warn("[catalog] fetchMore failed", {
            name: err instanceof Error ? err.name : "unknown",
            searchQuery,
            endCursor,
          });
          setFetchMoreError(t("fetchMoreError"));
        })
        .finally(() => {
          fetchingRef.current = false;
        });
    },
    [fetchMore, t],
  );

  const requestNextPageFromObserver = useEffectEvent(() => {
    fetchNextPage({ hasNextPage, endCursor, searchQuery });
  });

  useEffect(() => {
    // Halt the observer loop while a previous fetch failed; user must click Retry to resume.
    if (!hasNextPage || fetchMoreError != null) return;
    const node = sentinelRef.current;
    if (!node) return;

    const observer = new IntersectionObserver((entries) => {
      if (!entries[0]?.isIntersecting || fetchingRef.current) return;
      requestNextPageFromObserver();
    });

    observer.observe(node);
    return () => observer.disconnect();
  }, [hasNextPage, fetchMoreError]);

  const handleImport = useCallback(
    async (id: string) => {
      // Serialize: ignore a second click while another import is in flight.
      if (importingId !== null) return;
      setImportError(null);
      setImportAuthError(null);
      setImportingId(id);

      const outcome = await importMasterCardgroup(id);
      setImportingId(null);

      switch (outcome.status) {
        case "success":
          setImportedIds((prev) => {
            const next = new Set(prev);
            next.add(id);
            return next;
          });
          toast(t("importSuccess", { name: outcome.cardgroupName }));
          return;
        case "not_found":
          setImportError(t("importNotFound"));
          return;
        case "auth":
          setImportAuthError(outcome.kind);
          return;
        case "rejected":
          setImportError(t("importError"));
          return;
      }
    },
    [importingId, importMasterCardgroup, t],
  );

  const fetchingMore = networkStatus === NetworkStatus.fetchMore || (loading && edges.length > 0);
  const initialLoading = loading && edges.length === 0 && networkStatus !== NetworkStatus.fetchMore;
  const hasSearch = searchQuery !== null && searchQuery !== "";

  return (
    <ListingPageShell
      title={t("title")}
      description={t("description")}
      toolbar={
        <div className="mb-2">
          <input
            type="search"
            placeholder={t("searchPlaceholder")}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="w-full max-w-sm rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            aria-label={t("searchAriaLabel")}
            data-testid="catalog-search"
          />
        </div>
      }
    >
      {importAuthError ? (
        <div
          role="alert"
          data-testid="catalog-import-auth-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          <span>{t("sessionExpired")}</span>
          <Link href="/login" className="underline">
            {t("signInAgain")}
          </Link>
        </div>
      ) : null}

      {importError ? (
        <div
          role="alert"
          data-testid="catalog-import-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {importError}
        </div>
      ) : null}

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
        <ul
          className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3"
          data-testid="catalog-list"
        >
          {edges.map((edge) => (
            <CatalogCard
              key={edge.cursor}
              node={edge.node}
              importing={importingId === edge.node.id}
              imported={importedIds.has(edge.node.id)}
              onImport={handleImport}
            />
          ))}
        </ul>
      )}

      <div ref={sentinelRef} aria-hidden="true" data-testid="catalog-sentinel" />

      {fetchMoreError && (
        <div
          className="mt-3 flex flex-col items-center gap-2 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="catalog-fetch-more-error"
        >
          <span>{fetchMoreError}</span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              setFetchMoreError(null);
              fetchNextPage({ hasNextPage, endCursor, searchQuery });
            }}
          >
            {tCommon("retry")}
          </Button>
        </div>
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
  );
}
