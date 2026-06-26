"use client";

import { NetworkStatus } from "@apollo/client";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  type MergeFromCatalogOutcome,
  useMergeFromCatalog,
} from "@/app/cardgroups/[id]/use-merge-from-catalog";
import { CatalogCard } from "@/app/catalog/catalog-card";
import { CATALOG_DEFAULT_VARS, CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { ConnectionListFooter } from "@/components/layout/connection-list-footer";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { buttonVariants } from "@/components/ui/button";
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
import { EMPTY_PAGE_INFO } from "@/lib/pagination/empty-page-info";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";

type Connection = MasterCatalogQuery["masterCatalog"];
type CatalogEdge = Connection["edges"][number];
type CatalogPageInfo = Connection["pageInfo"];

type MergeBanner = Exclude<MergeFromCatalogOutcome, { status: "success" }>;

type SelectedDeck = {
  id: string;
  name: string;
};

export type MergeFromCatalogSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  targetCardgroupId: string;
  onMerged: (result: { addedCount: number; updatedCount: number }) => void;
};

const CATALOG_INITIAL = {
  edges: [] as CatalogEdge[],
  pageInfo: EMPTY_PAGE_INFO,
  totalCount: 0,
};

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

function bannerCopy(
  banner: MergeBanner,
  t: ReturnType<typeof useTranslations<"Cardgroups">>,
): string {
  if (banner.status === "not_found") return t("mergeNotFound");
  if (banner.status === "rejected") return t("mergeError");
  return banner.kind === "forbidden" ? t("noPermission") : t("sessionExpiredSignIn");
}

export function MergeFromCatalogSheet({
  open,
  onOpenChange,
  targetCardgroupId,
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

  const [selectedDeck, setSelectedDeck] = useState<SelectedDeck | null>(null);
  const [banner, setBanner] = useState<MergeBanner | null>(null);
  const [pendingMergeId, setPendingMergeId] = useState<string | null>(null);

  const resetTransientState = useCallback(() => {
    setSelectedDeck(null);
    setBanner(null);
    setPendingMergeId(null);
    clearSearch();
  }, [clearSearch]);

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) {
        resetTransientState();
      }
      onOpenChange(nextOpen);
    },
    [onOpenChange, resetTransientState],
  );

  useEffect(() => {
    if (!open) {
      resetTransientState();
    }
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
    // The sheet is always mounted by its parent (e.g. CardgroupHeader); only
    // query the catalog once it is actually opened so a user who never merges
    // pays no MasterCatalog round-trip on every detail-page render.
    skip: !open,
  });

  const initialLoading = loading && edges.length === 0 && networkStatus !== NetworkStatus.fetchMore;
  const catalogCards = useFragment(
    CatalogCardFieldsFragment,
    edges.map((edge) => edge.node),
  );
  const queryBannerError = getBackendErrorBanner(queryError);

  const handleSelect = useCallback(
    (id: string) => {
      if (pendingMergeId !== null) return;
      const card = catalogCards.find((item) => item.id === id);
      if (!card) return;
      setBanner(null);
      setSelectedDeck({ id, name: card.name });
    },
    [catalogCards, pendingMergeId],
  );

  const handleConfirm = useCallback(async () => {
    if (selectedDeck === null || pendingMergeId !== null) return;

    setBanner(null);
    setPendingMergeId(selectedDeck.id);
    const outcome = await mergeFromCatalog(selectedDeck.id);

    if (outcome.status === "success") {
      onMerged({ addedCount: outcome.addedCount, updatedCount: outcome.updatedCount });
      setSelectedDeck(null);
      setPendingMergeId(null);
      onOpenChange(false);
      return;
    }

    setBanner(outcome);
    setSelectedDeck(null);
    setPendingMergeId(null);
  }, [mergeFromCatalog, onMerged, onOpenChange, pendingMergeId, selectedDeck]);

  return (
    <>
      <FormSheet
        open={open}
        onOpenChange={handleOpenChange}
        title={t("mergeFromCatalogTitle")}
        size="lg"
      >
        <div className="flex flex-col gap-4">
          <input
            type="search"
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            placeholder={tCatalog("searchPlaceholder")}
            aria-label={tCatalog("searchAriaLabel")}
            data-testid="merge-from-catalog-search"
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />

          {banner ? (
            <ErrorBanner data-testid="merge-from-catalog-error">
              {bannerCopy(banner, t)}
            </ErrorBanner>
          ) : null}

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
            <ul className="grid gap-3 sm:grid-cols-2" data-testid="merge-from-catalog-list">
              {edges.map((edge) => (
                <CatalogCard
                  key={edge.cursor}
                  node={edge.node}
                  importing={pendingMergeId === edge.node.id}
                  imported={false}
                  onImport={handleSelect}
                  labels={{
                    action: t("mergeFromCatalog"),
                    inProgress: tCatalog("importing"),
                    done: tCatalog("imported"),
                  }}
                  testIdPrefix="merge-from-catalog"
                  className="min-w-0"
                />
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
      </FormSheet>

      <AlertDialog
        open={selectedDeck !== null}
        onOpenChange={(nextOpen) => {
          if (pendingMergeId !== null) return;
          if (!nextOpen) setSelectedDeck(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("mergeConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("mergeConfirmBody")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={pendingMergeId !== null}>
              {tCommon("cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              className={buttonVariants({ variant: "destructive" })}
              disabled={pendingMergeId !== null}
              onClick={(event) => {
                event.preventDefault();
                void handleConfirm();
              }}
            >
              {t("mergeConfirmAction")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
