"use client";

import { useTranslations } from "next-intl";
import { useCallback, useState } from "react";
import { toast } from "sonner";
import { useImportMaster } from "@/app/catalog/use-import-master";
import { CardSearchInput } from "@/components/cardgroups/card-search-input";
import { ReadOnlyCardRow } from "@/components/cardgroups/read-only-card-row";
import { ConnectionListFooter } from "@/components/layout/connection-list-footer";
import { SearchTakeoverBar } from "@/components/search/search-takeover-bar";
import { AuthErrorBanner } from "@/components/ui/auth-error-banner";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorBanner } from "@/components/ui/error-banner";
import {
  CatalogMasterCardsConnectionDocument,
  type CatalogMasterCardsConnectionQuery,
} from "@/generated/graphql";
import { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import { useSeedConnectionCache } from "@/lib/pagination/use-seed-connection-cache";
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
  const t = useTranslations("Catalog");
  const tCards = useTranslations("Cards");
  const tCommon = useTranslations("Common");

  const search = useHeaderTakeoverSearch();

  // Seed the SSR master-cards connection into the cache synchronously before
  // useQuery runs. catalogCardsDefaultVars(id) keeps the cache key identical to
  // the SSR seed and the client useQuery — any mismatch silently splits the
  // cache. The connection object is built here (where __typename is assembled);
  // initial props are required, so no warnScope/null-skip is needed.
  useSeedConnectionCache({
    document: CatalogMasterCardsConnectionDocument,
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

  const { edges, pageInfo, totalCount, fetchingMore, fetchMoreError, retryFetchMore, sentinelRef } =
    useCatalogCardsConnection({
      masterCardgroupId: id,
      searchQuery: search.query,
      initialEdges,
      initialPageInfo,
      initialTotalCount,
      // The deck-detail list loads CARDS, so reuse the Cards namespace ("load
      // more cards"), not Catalog.fetchMoreError ("load more cardgroups").
      fetchMoreErrorMessage: tCards("fetchMoreFailed"),
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

  // `deckId` is forwarded by CatalogImportButton (= initialDeck.id); honoring the
  // `onImport(id)` contract rather than closing over the route `id` keeps the
  // header's prop type truthful.
  const handleImport = useCallback(
    async (deckId: string) => {
      // Serialize: ignore a second click while another import is in flight or done.
      if (importing || imported) return;
      setImportError(null);
      setImportAuthError(null);
      setImporting(true);

      const outcome = await importMasterCardgroup(deckId);
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
    },
    [importing, imported, importMasterCardgroup, initialDeck.name, t],
  );

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
      <main className="p-8">
        <CatalogDeckHeader
          deck={initialDeck}
          cardCount={totalCount}
          importing={importing}
          imported={imported}
          onImport={handleImport}
        />

        <div className="space-y-3">
          {importAuthError ? (
            <AuthErrorBanner
              testId="catalog-deck-import-auth-error"
              message={t("sessionExpired")}
              signInLabel={t("signInAgain")}
            />
          ) : null}

          {importError ? (
            <ErrorBanner data-testid="catalog-deck-import-error">{importError}</ErrorBanner>
          ) : null}

          <CardSearchInput value={search.input} onChange={search.setInput} />

          {edges.length === 0 && search.query && (
            <EmptyState
              className="rounded-md p-6"
              testId="catalog-deck-empty-search"
              body={t("deckEmptySearch", { query: search.query })}
            />
          )}

          {edges.length === 0 && !search.query && (
            <EmptyState
              className="rounded-md p-6"
              testId="catalog-deck-empty"
              body={t("deckEmpty")}
            />
          )}

          {edges.length > 0 && (
            <ul className="space-y-3" data-testid="catalog-deck-card-list">
              {edges.map((edge) => (
                <li key={edge.node.id}>
                  <ReadOnlyCardRow card={edge.node} />
                </li>
              ))}
            </ul>
          )}

          <ConnectionListFooter
            sentinelRef={sentinelRef}
            hasNextPage={pageInfo.hasNextPage}
            fetchingMore={fetchingMore}
            fetchMoreError={fetchMoreError}
            onRetry={retryFetchMore}
            retryLabel={tCommon("retry")}
            loadingMoreLabel={tCards("loadingMore")}
            testIdPrefix="catalog-deck"
          />
        </div>
      </main>
    </>
  );
}
