"use client";

import { NetworkStatus } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { Plus } from "lucide-react";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import { CardgroupListItem } from "@/components/cardgroups/cardgroup-list-item";
import { CardgroupsToolbar } from "@/components/cardgroups/cardgroups-toolbar";
import { PaginatedPublicListScreen } from "@/components/layout/paginated-public-list-screen";
import { AuthErrorBanner } from "@/components/ui/auth-error-banner";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import {
  MyCardgroupsConnectionDocument,
  type MyCardgroupsConnectionQuery,
  type MyCardgroupsConnectionQueryVariables,
} from "@/generated/graphql";
import { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { FLAMINGO_EVENT, subscribeFlamingo } from "@/lib/events/flamingo-events";
import { EMPTY_PAGE_INFO } from "@/lib/pagination/empty-page-info";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";
import { useSeedConnectionCache } from "@/lib/pagination/use-seed-connection-cache";
import { useUndoDelete } from "@/lib/undo-delete";
import { removeMyCardgroupEdge, restoreMyCardgroupSnapshot } from "./cache";
import { CARDGROUPS_DEFAULT_VARS, DeleteCardgroupMutation } from "./queries";
import { useCreateCardgroupForm } from "./use-create-cardgroup-form";

type Connection = MyCardgroupsConnectionQuery["myCardgroupsConnection"];
type CardgroupEdge = Connection["edges"][number];
type CardgroupPageInfo = Connection["pageInfo"];

interface CardgroupsClientProps {
  initialConnection: Connection | null;
}

// Render fallback for useConnectionPagination. The client seeds the cache
// synchronously before useQuery runs, so this is never read on the happy path;
// it keeps the empty-edges shape the inline implementation used (`?? []`).
const CARDGROUPS_INITIAL = {
  edges: [] as CardgroupEdge[],
  pageInfo: EMPTY_PAGE_INFO,
  totalCount: 0,
};

// Concatenate the next page's edges onto the cached cardgroups connection.
function mergeCardgroupsConnection(
  prev: MyCardgroupsConnectionQuery,
  more: MyCardgroupsConnectionQuery,
): MyCardgroupsConnectionQuery {
  return {
    myCardgroupsConnection: {
      ...more.myCardgroupsConnection,
      edges: [...prev.myCardgroupsConnection.edges, ...more.myCardgroupsConnection.edges],
    },
  };
}

function CreateCardgroupSheetContent({
  submit,
  submitting,
  validationError,
  authError,
  unexpectedError,
  limitError,
  onDirtyChange,
}: {
  submit: (values: { name: string }) => Promise<void>;
  submitting: boolean;
  validationError: { field: string; message: string } | null;
  authError: "unauthenticated" | "forbidden" | null;
  unexpectedError: string | null;
  limitError: string | null;
  onDirtyChange: (dirty: boolean) => void;
}) {
  const close = useFormSheetClose();
  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");

  return (
    <div className="space-y-4">
      {authError ? (
        <AuthErrorBanner
          testId="cardgroup-create-auth-error"
          message={authError === "unauthenticated" ? t("sessionExpired") : t("noPermission")}
          signInLabel={t("signInAgain")}
        />
      ) : null}

      {limitError ? (
        <ErrorBanner data-testid="cardgroup-create-limit-error">{limitError}</ErrorBanner>
      ) : null}

      {unexpectedError ? (
        <ErrorBanner data-testid="cardgroup-create-unexpected-error">{unexpectedError}</ErrorBanner>
      ) : null}

      <CardgroupForm
        mode="create"
        defaultValues={{ name: "" }}
        submit={submit}
        submitting={submitting}
        validationError={validationError}
        onDirtyChange={onDirtyChange}
        secondarySlot={
          <Button type="button" variant="outline" onClick={close}>
            {tCommon("cancel")}
          </Button>
        }
      />
    </div>
  );
}

/**
 * Client component for the /cardgroups listing page.
 *
 * Wires:
 *  - Debounced search (300ms; searchInput → searchQuery), passed as the
 *    `search` variable on MyCardgroupsConnectionQuery.
 *  - Infinite scroll via IntersectionObserver, with an in-flight guard via
 *    useRef<boolean> (per docs/pagination/intersection-observer-in-flight-guard.md).
 *  - fetchMoreError halt gate — observer short-circuits while an error banner
 *    is showing; user must click Retry to resume.
 *  - SSR seed: writes initialConnection into the cache once at mount via
 *    cache.writeQuery so useQuery (cache-first) renders immediately without
 *    a network round-trip.
 */
export default function CardgroupsClient({ initialConnection }: CardgroupsClientProps) {
  const apollo = useApolloClient();
  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");
  const search = useHeaderTakeoverSearch();
  const searchQuery = search.query;

  const [deleteCommitError, setDeleteCommitError] = useState<string | null>(null);

  const { scheduleDelete } = useUndoDelete();
  const [deleteCardgroup] = useMutation(DeleteCardgroupMutation);

  // Create-cardgroup drawer (FormSheet → bottom drawer on mobile, right panel
  // on desktop). Opened by the nav-header "+" button via the
  // flamingo:add-cardgroup event, by the desktop "New cardgroup" button, and
  // by the empty-state CTA. The full-page /cardgroups/new route stays for the
  // onboarding (welcome) and returnTo flows.
  const {
    submit: submitCreateCardgroup,
    loading: creating,
    validationError: addValidationError,
    authError: addAuthError,
    limitError,
    unexpectedError,
    reset: resetCreateForm,
  } = useCreateCardgroupForm();
  const [addOpen, setAddOpen] = useState(false);
  const [addDirty, setAddDirty] = useState(false);

  // Translate the hook's structured error state to display copy. The drawer is
  // modal, so surface a banner for both an unparseable payload ("unexpected")
  // and a transport failure ("rejected") rather than failing silently.
  const addLimitError = limitError
    ? t("limitReached", { limit: limitError.limit, current: limitError.current })
    : null;
  const addUnexpectedError = unexpectedError ? tCommon("somethingWentWrong") : null;

  const openAddSheet = useCallback(() => {
    resetCreateForm();
    setAddDirty(false);
    setAddOpen(true);
  }, [resetCreateForm]);

  useEffect(() => {
    // Cancel any default navigation to /cardgroups/new and open the drawer in
    // place instead.
    return subscribeFlamingo(FLAMINGO_EVENT.addCardgroup, (_detail, event) => {
      event.preventDefault();
      openAddSheet();
    });
  }, [openAddSheet]);

  async function handleCreateCardgroup(values: { name: string }) {
    const outcome = await submitCreateCardgroup(values);

    if (outcome.status === "success") {
      // The connection cache is updated inside useCreateCardgroup, so the new
      // row appears in the list without a refetch. Just close the drawer.
      setAddDirty(false);
      setAddOpen(false);
    }
  }

  // Seed the SSR connection into the cache synchronously before useQuery runs.
  // CARDGROUPS_DEFAULT_VARS keeps the cache key identical to the SSR seed and the
  // client useQuery — any mismatch silently splits the cache.
  useSeedConnectionCache({
    document: MyCardgroupsConnectionDocument,
    variables: CARDGROUPS_DEFAULT_VARS,
    data: { myCardgroupsConnection: initialConnection },
    warnScope: "cardgroups-client",
  });

  // When searchQuery is null we use CARDGROUPS_DEFAULT_VARS verbatim so the
  // cache key matches the SSR seed exactly. For non-null searches we spread and
  // override `search`, keeping `first` in sync with the default. Memoized so the
  // hook's useQuery does not re-subscribe on unrelated re-renders.
  const queryVariables = useMemo(
    () =>
      searchQuery === null
        ? CARDGROUPS_DEFAULT_VARS
        : { ...CARDGROUPS_DEFAULT_VARS, search: searchQuery },
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
    MyCardgroupsConnectionQuery,
    MyCardgroupsConnectionQueryVariables,
    CardgroupEdge,
    CardgroupPageInfo
  >({
    document: MyCardgroupsConnectionDocument,
    variables: queryVariables,
    searchQuery: searchQuery,
    selectConnection: (data) => data?.myCardgroupsConnection,
    buildFetchMoreVariables: (after, searchValue) => ({
      ...CARDGROUPS_DEFAULT_VARS,
      after,
      search: searchValue,
    }),
    mergeConnection: mergeCardgroupsConnection,
    initial: CARDGROUPS_INITIAL,
    resolveFetchMoreError: (err) => getBackendErrorBanner(err) ?? t("fetchMoreError"),
    logScope: "[cardgroups]",
  });
  const hasNextPage = pageInfo.hasNextPage;

  function handleDelete(id: string, name: string) {
    // Clear any stale delete-error banner so a new attempt starts clean.
    setDeleteCommitError(null);
    // Use queryVariables (the active search variables) so the cache key matches
    // the currently rendered query. Using CARDGROUPS_DEFAULT_VARS here would
    // silently read/write the wrong cache entry when a search is active.
    const activeVars = queryVariables;
    const snapshot = removeMyCardgroupEdge(apollo.cache, id, activeVars);
    if (!snapshot) {
      console.warn("[cardgroups] handleDelete: cache miss on snapshot read", { id });
      setDeleteCommitError(t("deleteReloadError"));
      return;
    }

    scheduleDelete({
      id,
      label: t("cardgroupDeleted", { name }),
      optimisticRollback: () => {
        restoreMyCardgroupSnapshot(apollo.cache, snapshot, activeVars);
      },
      commitDelete: async () => {
        await deleteCardgroup({ variables: { id } });
        apollo.cache.evict({ id: apollo.cache.identify({ __typename: "Cardgroup", id }) });
        apollo.cache.gc();
      },
      onCommitFailed: (err) => {
        setDeleteCommitError(getBackendErrorBanner(err) ?? t("deleteReloadError"));
      },
    });
  }

  const initialLoading = loading && edges.length === 0 && networkStatus !== NetworkStatus.fetchMore;
  const hasSearch = searchQuery !== null && searchQuery !== "";

  return (
    <PaginatedPublicListScreen
      search={{
        search,
        placeholder: t("filterPlaceholder"),
        ariaLabel: t("filterAriaLabel"),
      }}
      desktopSearch={
        <CardgroupsToolbar searchInput={search.input} onSearchInputChange={search.setInput} />
      }
      title={t("myCardgroups")}
      count={totalCount}
      countLabel={tCommon("totalCount", { count: totalCount })}
      primaryActions={
        <Button
          type="button"
          variant="brand"
          className="hidden md:inline-flex"
          onClick={openAddSheet}
          data-testid="cardgroups-header-new-btn"
        >
          <span>{t("newCardgroup")}</span>
          <Plus aria-hidden="true" />
        </Button>
      }
      initialLoading={initialLoading}
      loadingLabel={tCommon("loading")}
      isEmpty={edges.length === 0}
      hasSearch={hasSearch}
      emptyState={
        <div
          className="flex flex-col items-center gap-3 py-8 text-center"
          data-testid="cardgroups-empty"
        >
          <p className="text-sm text-muted-foreground">{t("noCardgroupsYet")}</p>
          <Button
            type="button"
            variant="brand"
            onClick={openAddSheet}
            data-testid="cardgroups-empty-cta"
          >
            <span>{t("newCardgroup")}</span>
            <Plus aria-hidden="true" />
          </Button>
        </div>
      }
      emptySearchState={
        hasSearch ? (
          <p className="text-sm text-muted-foreground" data-testid="cardgroups-empty-search">
            {t("noMatch", { query: searchQuery })}
          </p>
        ) : null
      }
      footer={{
        sentinelRef,
        fetchMoreError,
        onRetry: retryFetchMore,
        fetchingMore,
        hasNextPage,
        retryLabel: tCommon("retry"),
        loadingMoreLabel: t("loadingMore"),
      }}
      testIdPrefix="cardgroups"
    >
      {edges.length > 0 && (
        <ul className="space-y-3" data-testid="cardgroups-list">
          {edges.map((edge) => (
            <CardgroupListItem
              key={edge.node.id}
              id={edge.node.id}
              name={edge.node.name}
              updatedAt={edge.node.updatedAt as string}
              onDelete={handleDelete}
            />
          ))}
        </ul>
      )}

      {deleteCommitError && (
        <ErrorBanner className="mt-3" data-testid="cardgroups-delete-error">
          {deleteCommitError}
        </ErrorBanner>
      )}

      <FormSheet
        title={t("newCardgroup")}
        open={addOpen}
        onOpenChange={(nextOpen) => {
          setAddOpen(nextOpen);
          if (!nextOpen) {
            setAddDirty(false);
            resetCreateForm();
          }
        }}
        submitting={creating}
        dirty={addDirty}
        confirmOnDismiss
        size="sm"
      >
        <CreateCardgroupSheetContent
          submit={handleCreateCardgroup}
          submitting={creating}
          validationError={addValidationError}
          authError={addAuthError}
          unexpectedError={addUnexpectedError}
          limitError={addLimitError}
          onDirtyChange={setAddDirty}
        />
      </FormSheet>
    </PaginatedPublicListScreen>
  );
}
