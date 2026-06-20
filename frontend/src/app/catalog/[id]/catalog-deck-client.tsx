"use client";

import { useApolloClient } from "@apollo/client/react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useRef, useState } from "react";
import { toast } from "sonner";
import { useImportMaster } from "@/app/catalog/use-import-master";
import { CardFetchMoreError } from "@/components/cardgroups/card-fetch-more-error";
import { CardSearchInput } from "@/components/cardgroups/card-search-input";
import { ReadOnlyCardRow } from "@/components/cardgroups/read-only-card-row";
import { SearchTakeoverBar } from "@/components/search/search-takeover-bar";
import { ErrorBanner } from "@/components/ui/error-banner";
import {
  CatalogMasterCardsConnectionDocument,
  type CatalogMasterCardsConnectionQuery,
} from "@/generated/graphql";
import { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import type { CatalogDeck } from "./catalog-deck-header";
import { CatalogDeckHeader } from "./catalog-deck-header";
import { catalogCardsDefaultVars } from "./queries";
import { useCatalogCardsConnection } from "./use-catalog-cards-connection";

type Connection = CatalogMasterCardsConnectionQuery["masterCardsConnection"];
type CatalogCardEdge = Connection["edges"][number];
type CatalogCardPageInfo = Connection["pageInfo"];

interface CatalogDeckClientProps {
  id: string;
  initialDeck: CatalogDeck;
  initialEdges: CatalogCardEdge[];
  initialPageInfo: CatalogCardPageInfo;
  initialTotalCount: number;
}

/**
 * Client orchestrator for the public catalog deck-detail view (`/catalog/[id]`).
 * A read-only mirror of the cardgroup edit screen minus all edit chrome:
 *
 *  - SSR seed: writes the initial master-cards connection into the cache once
 *    synchronously during render (before useQuery runs) via apollo.writeQuery so
 *    the cache-first useQuery renders immediately without a network round-trip.
 *    catalogCardsDefaultVars keeps the cache key identical to the SSR seed and
 *    the client useQuery (a mismatch silently splits the cache). The deck
 *    metadata is static for the page lifetime, so it is consumed directly from
 *    the `initialDeck` prop rather than seeded + re-read.
 *  - Debounced header-takeover search (mobile) + desktop CardSearchInput, both
 *    feeding the `search` variable on the connection query.
 *  - Infinite scroll via the shared useConnectionPagination hook (IntersectionObserver
 *    + in-flight guard + fetchMore error halt gate).
 *  - "Import this deck" CTA in the header via useImportMaster (the only mutation;
 *    no card create/update/delete, no selection, no sheets).
 */
export default function CatalogDeckClient({
  id,
  initialDeck,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
}: CatalogDeckClientProps) {
  const apollo = useApolloClient();
  const t = useTranslations("Catalog");
  const tCards = useTranslations("Cards");

  const search = useHeaderTakeoverSearch();

  // Strict Mode double-mount safety: only write the SSR seed into the cache once.
  // The seededRef guard is synchronous, so it survives the double-invoke without
  // a second write. Doing this in a useEffect would create a window between first
  // paint and the post-render write where useQuery sees an empty cache.
  const seededRef = useRef(false);
  if (!seededRef.current) {
    seededRef.current = true;
    apollo.writeQuery({
      query: CatalogMasterCardsConnectionDocument,
      variables: catalogCardsDefaultVars(id),
      data: {
        masterCardsConnection: {
          __typename: "MasterCardConnection",
          edges: initialEdges,
          pageInfo: initialPageInfo,
          totalCount: initialTotalCount,
        },
      },
    });
  }

  const { edges, pageInfo, totalCount, fetchingMore, fetchMoreError, retryFetchMore, sentinelRef } =
    useCatalogCardsConnection({
      masterCardgroupId: id,
      searchQuery: search.query,
      initialEdges,
      initialPageInfo,
      initialTotalCount,
    });

  // Import state for this single deck. `importing` serializes; `imported` drives
  // the done affordance on the header CTA.
  const { importMasterCardgroup } = useImportMaster();
  const [importing, setImporting] = useState(false);
  const [imported, setImported] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const [importAuthError, setImportAuthError] = useState<"unauthenticated" | "forbidden" | null>(
    null,
  );

  const handleImport = useCallback(async () => {
    // Serialize: ignore a second click while another import is in flight or done.
    if (importing || imported) return;
    setImportError(null);
    setImportAuthError(null);
    setImporting(true);

    const outcome = await importMasterCardgroup(id);
    setImporting(false);

    switch (outcome.status) {
      case "success":
        setImported(true);
        toast(t("importSuccess", { name: initialDeck.name }));
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
  }, [importing, imported, importMasterCardgroup, id, initialDeck.name, t]);

  return (
    <>
      <SearchTakeoverBar
        open={search.searchOpen}
        value={search.input}
        onChange={search.setInput}
        onClear={search.clear}
        onClose={search.closeSearch}
        placeholder={tCards("searchPlaceholder")}
        ariaLabel={tCards("searchAriaLabel")}
      />
      <main className="p-4 md:p-8">
        <CatalogDeckHeader
          deck={initialDeck}
          cardCount={totalCount}
          importing={importing}
          imported={imported}
          onImport={handleImport}
        />

        <div className="space-y-3">
          {importAuthError ? (
            <ErrorBanner data-testid="catalog-deck-import-auth-error">
              <span>{t("sessionExpired")}</span>
              <Link href="/login" className="underline">
                {t("signInAgain")}
              </Link>
            </ErrorBanner>
          ) : null}

          {importError ? (
            <ErrorBanner data-testid="catalog-deck-import-error">{importError}</ErrorBanner>
          ) : null}

          <CardSearchInput value={search.input} onChange={search.setInput} />

          {edges.length === 0 ? (
            search.query ? (
              <div
                className="flex flex-col items-center gap-3 rounded-md border border-dashed border-border p-6"
                data-testid="catalog-deck-empty-search"
              >
                <p className="text-sm text-muted-foreground">
                  {t("deckEmptySearch", { query: search.query })}
                </p>
              </div>
            ) : (
              <div
                className="flex flex-col items-center gap-3 rounded-md border border-dashed border-border p-6"
                data-testid="catalog-deck-empty"
              >
                <p className="text-sm text-muted-foreground">{t("deckEmpty")}</p>
              </div>
            )
          ) : (
            <ul
              className="divide-y divide-border overflow-hidden rounded-md border border-border"
              data-testid="catalog-deck-card-list"
            >
              {edges.map((edge) => (
                <li key={edge.node.id}>
                  <ReadOnlyCardRow card={edge.node} />
                </li>
              ))}
            </ul>
          )}

          <div ref={sentinelRef} aria-hidden="true" data-testid="catalog-deck-sentinel" />

          {fetchMoreError && (
            <CardFetchMoreError message={fetchMoreError} onRetry={retryFetchMore} />
          )}

          {!fetchMoreError && fetchingMore && pageInfo.hasNextPage && (
            <p
              className="mt-3 text-center text-xs text-muted-foreground"
              data-testid="catalog-deck-loading-more"
            >
              {tCards("loadingMore")}
            </p>
          )}
        </div>
      </main>
    </>
  );
}
