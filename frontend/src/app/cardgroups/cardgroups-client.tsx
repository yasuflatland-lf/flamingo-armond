"use client";

import { NetworkStatus } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { Plus } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CardgroupForm } from "@/components/cardgroups/cardgroup-form";
import { CardgroupListItem } from "@/components/cardgroups/cardgroup-list-item";
import { CardgroupsToolbar } from "@/components/cardgroups/cardgroups-toolbar";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { SearchTakeoverBar } from "@/components/search/search-takeover-bar";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import {
  MyCardgroupsConnectionDocument,
  type MyCardgroupsConnectionQuery,
  type MyCardgroupsConnectionQueryVariables,
} from "@/generated/graphql";
import { useDebouncedSearch } from "@/hooks/use-debounced-search";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";
import { useUndoDelete } from "@/lib/undo-delete";
import { CARDGROUPS_DEFAULT_VARS, DeleteCardgroupMutation } from "./queries";
import { useCreateCardgroup } from "./use-create-cardgroup";

type Connection = MyCardgroupsConnectionQuery["myCardgroupsConnection"];
type CardgroupEdge = Connection["edges"][number];
type CardgroupPageInfo = Connection["pageInfo"];

interface CardgroupsClientProps {
  initialConnection: Connection | null;
}

const EMPTY_PAGE_INFO: CardgroupPageInfo = {
  __typename: "PageInfo",
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: null,
  endCursor: null,
};

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
  onDirty,
}: {
  submit: (values: { name: string }) => Promise<void>;
  submitting: boolean;
  validationError: { field: string; message: string } | null;
  authError: "unauthenticated" | "forbidden" | null;
  unexpectedError: string | null;
  limitError: string | null;
  onDirty: () => void;
}) {
  const close = useFormSheetClose();
  const t = useTranslations("Cardgroups");
  const tCommon = useTranslations("Common");

  return (
    <div onInput={onDirty} className="space-y-4">
      {authError ? (
        <ErrorBanner data-testid="cardgroup-create-auth-error">
          <span>{authError === "unauthenticated" ? t("sessionExpired") : t("noPermission")}</span>
          <Link href="/login" className="underline">
            {t("signInAgain")}
          </Link>
          .
        </ErrorBanner>
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
  const search = useDebouncedSearch();
  const searchQuery = search.query;
  const [searchOpen, setSearchOpen] = useState(false);

  // The nav-header search trigger dispatches this; open the takeover in place.
  useEffect(() => {
    function handleOpenSearch() {
      setSearchOpen(true);
    }
    window.addEventListener("flamingo:open-search", handleOpenSearch);
    return () => window.removeEventListener("flamingo:open-search", handleOpenSearch);
  }, []);

  // Report filter state back to the header trigger (active dot + aria-expanded).
  useEffect(() => {
    window.dispatchEvent(
      new CustomEvent("flamingo:search-state", {
        detail: { active: searchQuery !== null && searchQuery !== "", visible: searchOpen },
      }),
    );
  }, [searchQuery, searchOpen]);

  // Reset the header trigger when leaving /cardgroups.
  useEffect(() => {
    return () => {
      window.dispatchEvent(
        new CustomEvent("flamingo:search-state", {
          detail: { active: false, visible: false },
        }),
      );
    };
  }, []);

  const closeSearch = useCallback(() => setSearchOpen(false), []);

  const [deleteCommitError, setDeleteCommitError] = useState<string | null>(null);

  const { scheduleDelete } = useUndoDelete();
  const [deleteCardgroup] = useMutation(DeleteCardgroupMutation);

  // Create-cardgroup drawer (FormSheet → bottom drawer on mobile, right panel
  // on desktop). Opened by the nav-header "+" button via the
  // flamingo:add-cardgroup event, by the desktop "New cardgroup" button, and
  // by the empty-state CTA. The full-page /cardgroups/new route stays for the
  // onboarding (welcome) and returnTo flows.
  const { create: createCardgroup, loading: creating } = useCreateCardgroup();
  const [addOpen, setAddOpen] = useState(false);
  const [addDirty, setAddDirty] = useState(false);
  const [addValidationError, setAddValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);
  const [addAuthError, setAddAuthError] = useState<"unauthenticated" | "forbidden" | null>(null);
  const [addUnexpectedError, setAddUnexpectedError] = useState<string | null>(null);
  const [addLimitError, setAddLimitError] = useState<string | null>(null);

  const openAddSheet = useCallback(() => {
    setAddValidationError(null);
    setAddAuthError(null);
    setAddUnexpectedError(null);
    setAddLimitError(null);
    setAddDirty(false);
    setAddOpen(true);
  }, []);

  useEffect(() => {
    function handleAddCardgroupEvent(event: Event) {
      // Cancel any default navigation to /cardgroups/new and open the
      // drawer in place instead.
      event.preventDefault();
      openAddSheet();
    }
    window.addEventListener("flamingo:add-cardgroup", handleAddCardgroupEvent);
    return () => window.removeEventListener("flamingo:add-cardgroup", handleAddCardgroupEvent);
  }, [openAddSheet]);

  async function handleCreateCardgroup(values: { name: string }) {
    setAddValidationError(null);
    setAddAuthError(null);
    setAddUnexpectedError(null);
    setAddLimitError(null);

    const outcome = await createCardgroup(values.name);

    switch (outcome.status) {
      case "validation":
        setAddValidationError({ field: outcome.field, message: outcome.message });
        return;
      case "auth":
        setAddAuthError(outcome.kind);
        return;
      case "limit":
        setAddLimitError(t("limitReached", { limit: outcome.limit, current: outcome.current }));
        return;
      case "unexpected":
      case "rejected":
        // The drawer is modal, so surface a banner for both an unparseable
        // payload and a transport failure rather than failing silently.
        setAddUnexpectedError(tCommon("somethingWentWrong"));
        return;
      case "success":
        // The connection cache is updated inside useCreateCardgroup, so the new
        // row appears in the list without a refetch. Just close the drawer.
        setAddDirty(false);
        setAddOpen(false);
        return;
    }
  }

  // Strict Mode double-mount safety: only write the SSR seed into the cache once.
  const seededRef = useRef(false);

  // Seed the cache synchronously during render (before useQuery runs) with the
  // SSR initialConnection so the first useQuery pass (cache-first) finds the
  // data already in the cache and renders without a network round-trip. Doing
  // this in a useEffect would create a window between first paint and the
  // post-render write where useQuery sees an empty cache.
  // CARDGROUPS_DEFAULT_VARS keeps the cache key identical to the SSR seed and
  // the client useQuery — any mismatch silently splits the cache.
  // The seededRef guard is synchronous, so it survives Strict Mode's
  // double-invoke without producing a second write.
  if (!seededRef.current && initialConnection === null) {
    console.warn(
      "[cardgroups-client] initialConnection is null — SSR seed skipped; useQuery will fetch fresh",
    );
  }
  if (!seededRef.current && initialConnection != null) {
    seededRef.current = true;
    apollo.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: { myCardgroupsConnection: initialConnection },
    });
  }

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
    const snapshot = apollo.cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: activeVars,
    });
    if (!snapshot) {
      console.warn("[cardgroups] handleDelete: cache miss on snapshot read", { id });
      setDeleteCommitError(t("deleteReloadError"));
      return;
    }

    apollo.cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: activeVars,
      data: {
        myCardgroupsConnection: {
          ...snapshot.myCardgroupsConnection,
          edges: snapshot.myCardgroupsConnection.edges.filter((e) => e.node.id !== id),
          totalCount: Math.max(0, snapshot.myCardgroupsConnection.totalCount - 1),
        },
      },
    });

    scheduleDelete({
      id,
      label: `Cardgroup "${name}" deleted`,
      optimisticRollback: () => {
        apollo.cache.writeQuery({
          query: MyCardgroupsConnectionDocument,
          variables: activeVars,
          data: snapshot,
        });
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
    <>
      <SearchTakeoverBar
        open={searchOpen}
        value={search.input}
        onChange={search.setInput}
        onClear={search.clear}
        onClose={closeSearch}
        placeholder={t("filterPlaceholder")}
        ariaLabel={t("filterAriaLabel")}
      />
      <ListingPageShell
        title={t("myCardgroups")}
        description={t("browseManage")}
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
        toolbar={
          <CardgroupsToolbar searchInput={search.input} onSearchInputChange={search.setInput} />
        }
      >
        {initialLoading && (
          <p className="text-sm text-muted-foreground" data-testid="cardgroups-loading">
            {tCommon("loading")}
          </p>
        )}

        {!initialLoading && edges.length === 0 && !hasSearch && (
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
        )}

        {!initialLoading && edges.length === 0 && hasSearch && (
          <p className="text-sm text-muted-foreground" data-testid="cardgroups-empty-search">
            {t("noMatch", { query: searchQuery })}
          </p>
        )}

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

        <div ref={sentinelRef} aria-hidden="true" data-testid="cardgroups-sentinel" />

        {fetchMoreError && (
          <ErrorBanner
            className="mt-3 flex flex-col items-center gap-2"
            data-testid="cardgroups-fetch-more-error"
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
            data-testid="cardgroups-loading-more"
          >
            {t("loadingMore")}
          </p>
        )}

        <FormSheet
          title={t("newCardgroup")}
          open={addOpen}
          onOpenChange={(nextOpen) => {
            setAddOpen(nextOpen);
            if (!nextOpen) {
              setAddDirty(false);
              setAddValidationError(null);
              setAddAuthError(null);
              setAddUnexpectedError(null);
              setAddLimitError(null);
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
            onDirty={() => setAddDirty(true)}
          />
        </FormSheet>
      </ListingPageShell>
    </>
  );
}
