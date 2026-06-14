"use client";

import { NetworkStatus } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import type { Reference } from "@apollo/client/utilities";
import { Plus } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Button } from "@/components/ui/button";
import { FormSheet } from "@/components/ui/form-sheet";
import { Input } from "@/components/ui/input";
import type {
  AdminMastersQuery as AdminMastersQueryResult,
  AdminMastersQueryVariables,
} from "@/generated/graphql";
import { classifyQueryError, getBackendErrorBanner, mutationAuthBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import { AdminMasterForm, type MasterFormValues } from "./admin-master-form";
import { AdminMasterRow } from "./admin-master-row";
import { AdminMastersSkeleton } from "./admin-masters-skeleton";
import {
  ADMIN_MASTERS_PAGE_SIZE,
  AdminCreateMasterMutation,
  AdminDeleteMasterMutation,
  AdminMastersQuery,
  AdminPublishMasterMutation,
  AdminUnpublishMasterMutation,
  AdminUpdateMasterMutation,
} from "./queries";

type Connection = AdminMastersQueryResult["adminMasters"];
type Edge = Connection["edges"][number];
type PageInfo = Connection["pageInfo"];

const BASE_VARS = {
  first: ADMIN_MASTERS_PAGE_SIZE,
  orderBy: "SORT_ORDER" as const,
  orderDirection: "ASC" as const,
};

const EMPTY_PAGE_INFO: PageInfo = {
  __typename: "PageInfo",
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: null,
  endCursor: null,
};

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
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState<string | null>(null);
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

  // Debounce search 300ms after the last keystroke.
  useEffect(() => {
    const timer = setTimeout(() => setSearchQuery(searchInput.trim() || null), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // The cache key is BASE_VARS + the active search; the create handler reads and
  // writes the search=null variant. Memoize on searchQuery so the hook's
  // useQuery does not re-subscribe on unrelated re-renders.
  const queryVariables = useMemo<AdminMastersQueryVariables>(
    () => ({ ...BASE_VARS, search: searchQuery }),
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
    buildFetchMoreVariables: (after, search) => ({ ...BASE_VARS, after, search }),
    mergeConnection: mergeMastersConnection,
    initial: MASTERS_INITIAL,
    resolveFetchMoreError: (err) => getBackendErrorBanner(err) ?? t("fetchMoreFailed"),
    logScope: "[admin-masters]",
  });
  const hasNextPage = pageInfo.hasNextPage;

  const queryErrorKind = classifyQueryError(queryError);
  const queryBannerError = queryErrorKind?.kind === "banner" ? queryErrorKind.message : undefined;

  const apolloClient = useApolloClient();
  const [runCreate, { loading: creating, reset: resetCreate }] =
    useMutation(AdminCreateMasterMutation);
  const [runUpdate, { loading: updating, reset: resetUpdate }] =
    useMutation(AdminUpdateMasterMutation);
  const [runDelete] = useMutation(AdminDeleteMasterMutation);
  const [runPublish] = useMutation(AdminPublishMasterMutation);
  const [runUnpublish] = useMutation(AdminUnpublishMasterMutation);

  const editId = sheet.state.mode === "edit" ? sheet.state.id : null;
  // Resolve the edit target by node.id, never by `cursor`: the backend emits an
  // opaque encoded cursor ("v1:..."), so matching against the raw id always misses
  // and renders "Master not found." for every master.
  const editEdge = editId ? edges.find((e) => e.node.id === editId) : undefined;

  // Create: prepend the new edge to the base (search=null) connection.
  const handleCreate = useCallback(
    async (values: MasterFormValues) => {
      setCreateValidationError(null);
      const result = await runCreate({ variables: { input: values } }).catch((err) => {
        const codes = liftGraphQLCodes(err);
        console.warn("[admin-masters] create rejected", {
          name: err instanceof Error ? err.name : "unknown",
          codes,
        });
        toast.error(
          mutationAuthBanner(err, {
            forbidden: t("forbidden"),
            unauthenticated: t("unauthenticated"),
            fallback: t("unexpectedError"),
          }),
        );
        return null;
      });
      if (!result) return;
      const payload = result.data?.adminCreateMasterCardgroup;
      const typename = payload?.__typename ?? "null";
      if (payload?.__typename === "InputValidationError") {
        setCreateValidationError({ field: payload.field, message: payload.message });
        return;
      }
      if (payload?.__typename === "CreateMasterCardgroupSuccess") {
        const created = payload.master;
        const vars = { ...BASE_VARS, search: null };
        const existing = apolloClient.cache.readQuery<AdminMastersQueryResult>({
          query: AdminMastersQuery,
          variables: vars,
        });
        const createdEdge = {
          __typename: "MasterCatalogEdge" as const,
          cursor: created.id,
          node: created,
        };
        if (existing?.adminMasters) {
          apolloClient.cache.writeQuery<AdminMastersQueryResult>({
            query: AdminMastersQuery,
            variables: vars,
            data: {
              adminMasters: {
                ...existing.adminMasters,
                edges: [createdEdge, ...existing.adminMasters.edges],
                totalCount: existing.adminMasters.totalCount + 1,
              },
            },
          });
        } else {
          apolloClient.cache.writeQuery<AdminMastersQueryResult>({
            query: AdminMastersQuery,
            variables: vars,
            data: {
              adminMasters: {
                __typename: "MasterCatalogConnection",
                edges: [createdEdge],
                pageInfo: {
                  __typename: "PageInfo",
                  hasNextPage: false,
                  hasPreviousPage: false,
                  startCursor: created.id,
                  endCursor: created.id,
                },
                totalCount: 1,
              },
            },
          });
        }
        toast.success(t("createSuccess"));
        resetCreate();
        setCreateDirty(false);
        sheet.close({ refresh: true });
        return;
      }
      // Neither known variant matched (null or an unknown union member) — never fail silently.
      console.warn("[admin-masters] unexpected createMaster payload", { typename });
      toast.error(t("unexpectedError"));
    },
    [apolloClient, runCreate, resetCreate, sheet, t],
  );

  // Update: normalization by id propagates the new fields; no manual cache write.
  const handleUpdate = useCallback(
    async (values: MasterFormValues) => {
      if (!editId) return;
      setEditValidationError(null);
      const result = await runUpdate({ variables: { id: editId, input: values } }).catch((err) => {
        const codes = liftGraphQLCodes(err);
        console.warn("[admin-masters] update rejected", {
          masterId: editId,
          name: err instanceof Error ? err.name : "unknown",
          codes,
        });
        toast.error(
          mutationAuthBanner(err, {
            forbidden: t("forbidden"),
            unauthenticated: t("unauthenticated"),
            fallback: t("unexpectedError"),
          }),
        );
        return null;
      });
      if (!result) return;
      const payload = result.data?.adminUpdateMasterCardgroup;
      const typename = payload?.__typename ?? "null";
      if (payload?.__typename === "InputValidationError") {
        setEditValidationError({ field: payload.field, message: payload.message });
        return;
      }
      if (payload?.__typename === "UpdateMasterCardgroupSuccess") {
        toast.success(t("updateSuccess"));
        resetUpdate();
        sheet.close({ refresh: true });
        return;
      }
      console.warn("[admin-masters] unexpected updateMaster payload", { typename });
      toast.error(t("unexpectedError"));
    },
    [editId, runUpdate, resetUpdate, sheet, t],
  );

  // Delete: filter the edge from every cached adminMasters variant, decrement
  // totalCount, evict the entity. Catches its own errors (toast) so the form's
  // confirm handler never sees a rejection.
  const handleDelete = useCallback(
    async (id: string) => {
      try {
        const result = await runDelete({ variables: { id } });
        if (!result.data?.adminDeleteMasterCardgroup) {
          throw new Error("adminDeleteMasterCardgroup returned false");
        }
        apolloClient.cache.modify({
          fields: {
            adminMasters(existing, { readField }) {
              const conn = existing as {
                edges?: ReadonlyArray<{ node: Reference }>;
                totalCount?: number;
              };
              if (!conn.edges) return existing;
              // Filter by the normalized node's id, not by `cursor`: in the cache the
              // edge cursor is the opaque encoded value ("v1:..."), so `cursor !== id`
              // never matches and the deleted edge would linger in the connection.
              const next = conn.edges.filter((edge) => readField<string>("id", edge.node) !== id);
              if (next.length === conn.edges.length) return existing;
              return { ...conn, edges: next, totalCount: Math.max(0, (conn.totalCount ?? 0) - 1) };
            },
          },
        });
        const cacheId = apolloClient.cache.identify({ __typename: "MasterCardgroup", id });
        if (cacheId) {
          apolloClient.cache.evict({ id: cacheId });
          apolloClient.cache.gc();
        }
        toast.success(t("deleteMasterSuccess"));
        sheet.close();
      } catch (err) {
        const codes = liftGraphQLCodes(err);
        console.warn("[admin-masters] delete rejected", {
          masterId: id,
          name: err instanceof Error ? err.name : "unknown",
          codes,
        });
        toast.error(
          mutationAuthBanner(err, {
            forbidden: t("forbidden"),
            unauthenticated: t("unauthenticated"),
            fallback: t("deleteMasterFailed"),
          }),
        );
      }
    },
    [apolloClient, runDelete, sheet, t],
  );

  // Publish / unpublish from the edit panel. Both mutations return the updated
  // node keyed by `id`, so Apollo normalization propagates the new status to the
  // list row badge and the form's `master.status` — no manual cache write needed.
  // Catches its own errors (toast) so the form's click handler never sees a
  // rejection beyond logging.
  const handlePublishToggle = useCallback(
    async (id: string, currentlyPublished: boolean) => {
      try {
        if (currentlyPublished) {
          const result = await runUnpublish({ variables: { id } });
          if (!result.data?.adminUnpublishMasterCardgroup) {
            console.warn("[admin-masters] unpublish returned null payload", { masterId: id });
            toast.error(t("unexpectedError"));
            return;
          }
          toast.success(t("unpublishSuccess"));
          return;
        }
        const result = await runPublish({ variables: { id } });
        const payload = result.data?.adminPublishMasterCardgroup;
        // Capture the typename before narrowing exhausts the union type below.
        const typename = payload?.__typename ?? "null";
        if (payload?.__typename === "PublishMasterCardgroupSuccess") {
          toast.success(t("publishSuccess"));
          return;
        }
        if (payload?.__typename === "MasterCardgroupEmptyError") {
          // Backstop: the form disables Publish for 0-card drafts, but cardCount
          // may be stale. Surface the backend's empty-deck rejection.
          toast.error(t("publishEmptyToast"));
          return;
        }
        // Unexpected payload shape (null or unknown variant) — never fail silently.
        console.warn("[admin-masters] publish returned unexpected payload", {
          masterId: id,
          typename,
        });
        toast.error(t("unexpectedError"));
      } catch (err) {
        const codes = liftGraphQLCodes(err);
        console.warn("[admin-masters] publish toggle failed", {
          masterId: id,
          name: err instanceof Error ? err.name : "unknown",
          codes,
        });
        toast.error(
          mutationAuthBanner(err, {
            forbidden: t("forbidden"),
            unauthenticated: t("unauthenticated"),
            fallback: t("unexpectedError"),
          }),
        );
      }
    },
    [runPublish, runUnpublish, t],
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
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          aria-label={t("searchLabel")}
        />
      </div>

      {queryErrorKind?.kind === "forbidden" && (
        <div
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-masters-query-error"
        >
          {t("viewForbidden")}
        </div>
      )}

      {queryErrorKind?.kind === "unauthenticated" && (
        <div
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-masters-query-error"
        >
          <span>{t("sessionExpired")}</span>{" "}
          <Link href="/login" className="underline">
            {t("pleaseSignInAgain")}
          </Link>
        </div>
      )}

      {queryBannerError && (
        <div
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-masters-query-error"
        >
          <span>{queryBannerError}</span>
          <button type="button" className="ml-3 underline" onClick={() => refetch()}>
            {tCommon("retry")}
          </button>
        </div>
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
        <div
          className="mt-3 flex flex-col items-center gap-2 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
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
        </div>
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
