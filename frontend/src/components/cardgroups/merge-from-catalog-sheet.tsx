"use client";

import { NetworkStatus } from "@apollo/client";
import { ChevronRight } from "lucide-react";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useMergeFromCatalog } from "@/app/cardgroups/[id]/use-merge-from-catalog";
import { useMergeFromCatalogPreview } from "@/app/cardgroups/[id]/use-merge-from-catalog-preview";
import {
  CATALOG_DEFAULT_VARS,
  CATALOG_INITIAL,
  CatalogDeckFieldsFragment,
  mergeCatalogConnection,
} from "@/app/catalog/queries";
import { MergeReviewPanel } from "@/components/cardgroups/merge-review-panel";
import { ConnectionListFooter } from "@/components/layout/connection-list-footer";
import { SearchInput } from "@/components/search/search-input";
import { ErrorBanner } from "@/components/ui/error-banner";
import { FormSheet } from "@/components/ui/form-sheet";
import { useFragment } from "@/generated/fragment-masking";
import {
  MasterCatalogDocument,
  type MasterCatalogQuery,
  type MasterCatalogQueryVariables,
} from "@/generated/graphql";
import { useDebouncedSearch } from "@/hooks/use-debounced-search";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";

type Connection = MasterCatalogQuery["masterCatalog"];
type CatalogEdge = Connection["edges"][number];
type CatalogPageInfo = Connection["pageInfo"];

type SelectedDeck = { id: string; name: string };
type ReviewState =
  | { phase: "loading" }
  | { phase: "ready"; addedCount: number; updatedCount: number }
  | { phase: "error"; message: string };

/** Outcome shape shared by `previewMerge` and `mergeFromCatalog`'s non-success variants. */
type MergeFailureOutcome =
  | { status: "not_found" }
  | { status: "auth"; kind: "unauthenticated" | "forbidden" }
  | { status: "rejected" };

/** Maps a non-success merge outcome to its localized review-error message. */
function mergeOutcomeMessage(
  outcome: MergeFailureOutcome,
  t: ReturnType<typeof useTranslations<"Cardgroups">>,
): string {
  if (outcome.status === "not_found") return t("mergeNotFound");
  if (outcome.status === "auth") {
    return outcome.kind === "forbidden" ? t("noPermission") : t("sessionExpiredSignIn");
  }
  return t("mergeError");
}

export type MergeFromCatalogSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  targetCardgroupId: string;
  targetCardgroupName: string;
  onMerged: (result: { addedCount: number; updatedCount: number }) => void;
};

export function MergeFromCatalogSheet({
  open,
  onOpenChange,
  targetCardgroupId,
  targetCardgroupName,
  onMerged,
}: MergeFromCatalogSheetProps) {
  const t = useTranslations("Cardgroups");
  const tCatalog = useTranslations("Catalog");
  const tCommon = useTranslations("Common");
  const {
    input: searchInput,
    query: searchQuery,
    setInput: setSearchInput,
    clear: clearSearch,
  } = useDebouncedSearch();

  const { mergeFromCatalog } = useMergeFromCatalog(targetCardgroupId);
  const { previewMerge } = useMergeFromCatalogPreview(targetCardgroupId);

  const [selectedDeck, setSelectedDeck] = useState<SelectedDeck | null>(null);
  const [review, setReview] = useState<ReviewState | null>(null);
  const [merging, setMerging] = useState(false);

  const resetTransientState = useCallback(() => {
    setSelectedDeck(null);
    setReview(null);
    setMerging(false);
    clearSearch();
  }, [clearSearch]);

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) resetTransientState();
      onOpenChange(nextOpen);
    },
    [onOpenChange, resetTransientState],
  );

  useEffect(() => {
    if (!open) resetTransientState();
  }, [open, resetTransientState]);

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
    queryError,
  } = useConnectionPagination<
    MasterCatalogQuery,
    MasterCatalogQueryVariables,
    CatalogEdge,
    CatalogPageInfo
  >({
    document: MasterCatalogDocument,
    variables: queryVariables,
    searchQuery,
    selectConnection: (data) => data?.masterCatalog,
    buildFetchMoreVariables: (after, searchValue) => ({
      ...CATALOG_DEFAULT_VARS,
      after,
      search: searchValue,
    }),
    mergeConnection: mergeCatalogConnection,
    initial: CATALOG_INITIAL,
    resolveFetchMoreError: () => tCatalog("fetchMoreError"),
    logScope: "[merge-from-catalog]",
    // Always mounted by its consumer; only query once opened (skip on a cold cache).
    skip: !open || selectedDeck !== null,
  });

  const initialLoading = loading && edges.length === 0 && networkStatus !== NetworkStatus.fetchMore;
  const decks = useFragment(
    CatalogDeckFieldsFragment,
    edges.map((edge) => edge.node),
  );
  const queryBannerError = getBackendErrorBanner(queryError);

  const handleSelect = useCallback(
    async (id: string, name: string) => {
      setSelectedDeck({ id, name });
      setReview({ phase: "loading" });
      const outcome = await previewMerge(id);
      if (outcome.status === "success") {
        setReview({
          phase: "ready",
          addedCount: outcome.addedCount,
          updatedCount: outcome.updatedCount,
        });
        return;
      }
      setReview({ phase: "error", message: mergeOutcomeMessage(outcome, t) });
    },
    [previewMerge, t],
  );

  const handleBack = useCallback(() => {
    setSelectedDeck(null);
    setReview(null);
  }, []);

  const handleConfirm = useCallback(async () => {
    if (selectedDeck === null || merging) return;
    setMerging(true);
    const outcome = await mergeFromCatalog(selectedDeck.id);
    if (outcome.status === "success") {
      onMerged({ addedCount: outcome.addedCount, updatedCount: outcome.updatedCount });
      resetTransientState();
      onOpenChange(false);
      return;
    }
    setReview({ phase: "error", message: mergeOutcomeMessage(outcome, t) });
    setMerging(false);
  }, [mergeFromCatalog, merging, onMerged, onOpenChange, resetTransientState, selectedDeck, t]);

  const inReview = selectedDeck !== null;

  return (
    <FormSheet
      open={open}
      onOpenChange={handleOpenChange}
      title={t("mergeFromCatalogTitle")}
      size="lg"
    >
      {inReview ? (
        review?.phase === "loading" ? (
          <p
            className="text-sm text-muted-foreground"
            data-testid="merge-from-catalog-preview-loading"
          >
            {t("mergePreviewLoading")}
          </p>
        ) : review?.phase === "error" ? (
          <div className="flex flex-col gap-4">
            <ErrorBanner data-testid="merge-from-catalog-error">{review.message}</ErrorBanner>
            <button
              type="button"
              onClick={handleBack}
              className="self-start text-sm text-muted-foreground underline"
              data-testid="merge-from-catalog-review-back"
            >
              {t("mergeBack")}
            </button>
          </div>
        ) : review?.phase === "ready" ? (
          <MergeReviewPanel
            catalogName={selectedDeck.name}
            destName={targetCardgroupName}
            addedCount={review.addedCount}
            updatedCount={review.updatedCount}
            merging={merging}
            onConfirm={handleConfirm}
            onBack={handleBack}
          />
        ) : null
      ) : (
        <div className="flex flex-col gap-4">
          <SearchInput
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            placeholder={tCatalog("searchPlaceholder")}
            aria-label={tCatalog("searchAriaLabel")}
            data-testid="merge-from-catalog-search"
          />

          {queryBannerError ? (
            <ErrorBanner data-testid="merge-from-catalog-query-error">
              {queryBannerError}
            </ErrorBanner>
          ) : null}

          {initialLoading ? (
            <p className="text-sm text-muted-foreground" data-testid="merge-from-catalog-loading">
              {tCommon("loading")}
            </p>
          ) : null}

          {!initialLoading && edges.length === 0 ? (
            <p className="text-sm text-muted-foreground" data-testid="merge-from-catalog-empty">
              {searchQuery ? tCatalog("noMatch", { query: searchQuery }) : tCatalog("noDecks")}
            </p>
          ) : null}

          {edges.length > 0 ? (
            <ul className="divide-y divide-border" data-testid="merge-from-catalog-list">
              {decks.map((deck) => (
                <li key={deck.id}>
                  <button
                    type="button"
                    onClick={() => void handleSelect(deck.id, deck.name)}
                    // `px-3` matches the search field's own horizontal padding so the deck
                    // name lines up with the placeholder text directly above it.
                    className="flex w-full items-center justify-between gap-3 px-3 py-3 text-left hover:bg-muted/40"
                    data-testid={`merge-from-catalog-row-${deck.id}`}
                  >
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-medium text-foreground">
                        {deck.name}
                      </span>
                      <span className="block text-xs text-muted-foreground">
                        {tCatalog("cardCount", { count: deck.cardCount })}
                      </span>
                    </span>
                    <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
                  </button>
                </li>
              ))}
            </ul>
          ) : null}

          <ConnectionListFooter
            sentinelRef={sentinelRef}
            fetchMoreError={fetchMoreError}
            onRetry={retryFetchMore}
            fetchingMore={fetchingMore}
            hasNextPage={pageInfo.hasNextPage}
            retryLabel={tCommon("retry")}
            loadingMoreLabel={tCatalog("loadingMore")}
            testIdPrefix="merge-from-catalog"
          />
        </div>
      )}
    </FormSheet>
  );
}
