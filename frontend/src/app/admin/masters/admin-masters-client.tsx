"use client";

import { NetworkStatus } from "@apollo/client";
import { Plus } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useMemo, useState } from "react";
import { toast } from "sonner";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { FormSheet } from "@/components/ui/form-sheet";
import { Input } from "@/components/ui/input";
import type {
  AdminMastersQuery as AdminMastersQueryResult,
  AdminMastersQueryVariables,
} from "@/generated/graphql";
import { useDebouncedSearch } from "@/hooks/use-debounced-search";
import { classifyQueryError, getBackendErrorBanner } from "@/lib/apollo/errors";
import { EMPTY_PAGE_INFO } from "@/lib/pagination/empty-page-info";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import { AdminMasterForm, type MasterFormValues } from "./admin-master-form";
import { AdminMasterRow } from "./admin-master-row";
import { AdminMastersSkeleton } from "./admin-masters-skeleton";
import { ADMIN_MASTERS_BASE_VARS, AdminMastersQuery } from "./queries";
import { type AuthKind, useMasterMutations } from "./use-master-mutations";

type Connection = AdminMastersQueryResult["adminMasters"];
type Edge = Connection["edges"][number];
type PageInfo = Connection["pageInfo"];

// Render fallback for useConnectionPagination before the first query resolves.
// Admin masters has no SSR seed, so this is the initial render value; it keeps
// the empty-edges shape the inline implementation used (`?? []`).
const MASTERS_INITIAL = {
  edges: [] as Edge[],
  pageInfo: EMPTY_PAGE_INFO,
  totalCount: 0,
};

// Concatenate the next page's edges onto the cached adminMasters connection.
function mergeMastersConnection(
  prev: AdminMastersQueryResult,
  more: AdminMastersQueryResult,
): AdminMastersQueryResult {
  return {
    adminMasters: {
      ...more.adminMasters,
      edges: [...prev.adminMasters.edges, ...more.adminMasters.edges],
    },
  };
}

export function AdminMastersClient() {
  const t = useTranslations("AdminMasters");
  const tCommon = useTranslations("Common");
  const search = useDebouncedSearch();
  const searchQuery = search.query;
  const [createDirty, setCreateDirty] = useState(false);
  const [createValidationError, setCreateValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);
  const [editValidationError, setEditValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);
  const sheet = useSheetSearchParam();

  // The cache key is ADMIN_MASTERS_BASE_VARS + the active search; the create handler reads and
  // writes the search=null variant. Memoize on searchQuery so the hook's
  // useQuery does not re-subscribe on unrelated re-renders.
  const queryVariables = useMemo<AdminMastersQueryVariables>(
    () => ({ ...ADMIN_MASTERS_BASE_VARS, search: searchQuery }),
    [searchQuery],
  );

  const {
    edges,
    pageInfo,
    totalCount,
    networkStatus,
    fetchingMore,
    fetchMoreError,
    retryFetchMore,
    sentinelRef,
    queryError,
    refetch,
  } = useConnectionPagination<AdminMastersQueryResult, AdminMastersQueryVariables, Edge, PageInfo>({
    document: AdminMastersQuery,
    variables: queryVariables,
    searchQuery,
    selectConnection: (data) => data?.adminMasters,
    buildFetchMoreVariables: (after, search) => ({ ...ADMIN_MASTERS_BASE_VARS, after, search }),
    mergeConnection: mergeMastersConnection,
    initial: MASTERS_INITIAL,
    resolveFetchMoreError: (err) => getBackendErrorBanner(err) ?? t("fetchMoreFailed"),
    logScope: "[admin-masters]",
  });
  const hasNextPage = pageInfo.hasNextPage;

  const queryErrorKind = classifyQueryError(queryError);
  const queryBannerError = queryErrorKind?.kind === "banner" ? queryErrorKind.message : undefined;

  const {
    createMaster,
    updateMaster,
    deleteMaster,
    publishMaster,
    unpublishMaster,
    creating,
    updating,
    resetCreate,
    resetUpdate,
  } = useMasterMutations();

  const editId = sheet.state.mode === "edit" ? sheet.state.id : null;
  // Resolve the edit target by node.id, never by `cursor`: the backend emits an
  // opaque encoded cursor ("v1:..."), so matching against the raw id always misses.
  const editEdge = editId ? edges.find((e) => e.node.id === editId) : undefined;

  const authToast = useCallback(
    (kind: AuthKind) => {
      toast.error(kind === "forbidden" ? t("forbidden") : t("unauthenticated"));
    },
    [t],
  );

  const handleCreate = useCallback(
    async (values: MasterFormValues) => {
      setCreateValidationError(null);
      const outcome = await createMaster(values);
      switch (outcome.status) {
        case "validation":
          setCreateValidationError({ field: outcome.field, message: outcome.message });
          return;
        case "success":
          toast.success(t("createSuccess"));
          resetCreate();
          setCreateDirty(false);
          sheet.close({ refresh: true });
          return;
        case "auth":
          authToast(outcome.kind);
          return;
        default:
          toast.error(t("unexpectedError"));
      }
    },
    [createMaster, resetCreate, sheet, t, authToast],
  );

  const handleUpdate = useCallback(
    async (values: MasterFormValues) => {
      if (!editId) return;
      setEditValidationError(null);
      const outcome = await updateMaster(editId, values);
      switch (outcome.status) {
        case "validation":
          setEditValidationError({ field: outcome.field, message: outcome.message });
          return;
        case "success":
          toast.success(t("updateSuccess"));
          resetUpdate();
          sheet.close({ refresh: true });
          return;
        case "auth":
          authToast(outcome.kind);
          return;
        default:
          toast.error(t("unexpectedError"));
      }
    },
    [editId, updateMaster, resetUpdate, sheet, t, authToast],
  );

  const handleDelete = useCallback(
    async (id: string) => {
      const outcome = await deleteMaster(id);
      switch (outcome.status) {
        case "success":
          toast.success(t("deleteMasterSuccess"));
          sheet.close();
          return;
        case "auth":
          authToast(outcome.kind);
          return;
        default:
          toast.error(t("deleteMasterFailed"));
      }
    },
    [deleteMaster, sheet, t, authToast],
  );

  const handlePublishToggle = useCallback(
    async (id: string, currentlyPublished: boolean) => {
      if (currentlyPublished) {
        const outcome = await unpublishMaster(id);
        switch (outcome.status) {
          case "success":
            toast.success(t("unpublishSuccess"));
            return;
          case "auth":
            authToast(outcome.kind);
            return;
          default:
            toast.error(t("unexpectedError"));
        }
        return;
      }
      const outcome = await publishMaster(id);
      switch (outcome.status) {
        case "success":
          toast.success(t("publishSuccess"));
          return;
        case "empty":
          toast.error(t("publishEmptyToast"));
          return;
        case "auth":
          authToast(outcome.kind);
          return;
        default:
          toast.error(t("unexpectedError"));
      }
    },
    [publishMaster, unpublishMaster, t, authToast],
  );

  const initialLoading = networkStatus === NetworkStatus.loading && edges.length === 0;

  if (initialLoading) return <AdminMastersSkeleton />;

  return (
    <ListingPageShell
      title={
        <span className="flex items-center gap-3">
          {t("title")}
          <span className="text-sm font-normal text-muted-foreground">({totalCount})</span>
        </span>
      }
      description={t("description")}
      primaryActions={
        <Button
          type="button"
          variant="brand"
          className="hidden md:inline-flex"
          data-testid="admin-masters-new-btn"
          onClick={() => sheet.open({ mode: "new" })}
        >
          <span>{t("newMaster")}</span>
          <Plus aria-hidden="true" />
        </Button>
      }
    >
      <div>
        <Input
          type="search"
          placeholder={t("searchPlaceholder")}
          value={search.input}
          onChange={(e) => search.setInput(e.target.value)}
          aria-label={t("searchLabel")}
        />
      </div>

      {queryErrorKind?.kind === "forbidden" && (
        <ErrorBanner data-testid="admin-masters-query-error">{t("viewForbidden")}</ErrorBanner>
      )}

      {queryErrorKind?.kind === "unauthenticated" && (
        <ErrorBanner data-testid="admin-masters-query-error">
          <span>{t("sessionExpired")}</span>{" "}
          <Link href="/login" className="underline">
            {t("pleaseSignInAgain")}
          </Link>
        </ErrorBanner>
      )}

      {queryBannerError && (
        <ErrorBanner data-testid="admin-masters-query-error">
          <span>{queryBannerError}</span>
          <button type="button" className="ml-3 underline" onClick={() => refetch()}>
            {tCommon("retry")}
          </button>
        </ErrorBanner>
      )}

      {!initialLoading && !queryErrorKind && edges.length === 0 && (
        <p className="text-sm text-muted-foreground" data-testid="admin-masters-empty">
          {t("noMastersFound")}
        </p>
      )}

      {edges.length > 0 && (
        <ul className="space-y-3" data-testid="admin-masters-list">
          {edges.map((edge) => (
            <AdminMasterRow
              key={edge.cursor}
              master={edge.node}
              onEdit={(id) => sheet.open({ mode: "edit", id })}
            />
          ))}
        </ul>
      )}

      <div ref={sentinelRef} aria-hidden="true" data-testid="admin-masters-sentinel" />

      {fetchMoreError && (
        <ErrorBanner
          className="mt-3 flex flex-col items-center gap-2"
          data-testid="admin-masters-fetch-more-error"
        >
          <span>{fetchMoreError}</span>
          <button
            type="button"
            className="rounded-md border border-destructive/40 px-3 py-1 text-xs hover:bg-destructive/10"
            onClick={retryFetchMore}
          >
            {tCommon("retry")}
          </button>
        </ErrorBanner>
      )}

      {!fetchMoreError && fetchingMore && hasNextPage && (
        <p
          className="mt-3 text-center text-xs text-muted-foreground"
          data-testid="admin-masters-loading-more"
        >
          {t("loadingMore")}
        </p>
      )}

      <FormSheet
        title={t("createMasterTitle")}
        open={sheet.state.mode === "new"}
        onOpenChange={(next) => {
          if (!next) {
            resetCreate();
            setCreateValidationError(null);
            setCreateDirty(false);
            sheet.close();
          }
        }}
        submitting={creating}
        dirty={createDirty}
        confirmOnDismiss
      >
        {sheet.state.mode === "new" ? (
          <AdminMasterForm
            mode="create"
            submitting={creating}
            submit={handleCreate}
            validationError={createValidationError}
            onDirtyChange={setCreateDirty}
          />
        ) : null}
      </FormSheet>

      <FormSheet
        title={t("editMasterTitle")}
        open={sheet.state.mode === "edit"}
        onOpenChange={(next) => {
          if (!next) {
            resetUpdate();
            setEditValidationError(null);
            sheet.close();
          }
        }}
        submitting={updating}
      >
        {sheet.state.mode === "edit" ? (
          editEdge ? (
            <AdminMasterForm
              mode="edit"
              master={editEdge.node}
              submitting={updating}
              submit={handleUpdate}
              validationError={editValidationError}
              onDelete={handleDelete}
              onPublishToggle={handlePublishToggle}
            />
          ) : (
            <div
              className="flex flex-col items-center justify-center gap-1 py-12 text-center"
              data-testid="admin-masters-edit-not-found"
            >
              <p className="text-sm font-medium text-muted-foreground">{t("masterNotFound")}</p>
            </div>
          )
        ) : null}
      </FormSheet>
    </ListingPageShell>
  );
}
