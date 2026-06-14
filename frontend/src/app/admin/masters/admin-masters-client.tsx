"use client";

import { NetworkStatus } from "@apollo/client";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import type { Reference } from "@apollo/client/utilities";
import { Plus } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useEffectEvent, useRef, useState } from "react";
import { toast } from "sonner";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Button } from "@/components/ui/button";
import { FormSheet } from "@/components/ui/form-sheet";
import { Input } from "@/components/ui/input";
import type { AdminMastersQuery as AdminMastersQueryResult } from "@/generated/graphql";
import { classifyQueryError, getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import type { FetchNextPageInput } from "@/lib/pagination/types";
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

const BASE_VARS = {
  first: ADMIN_MASTERS_PAGE_SIZE,
  orderBy: "SORT_ORDER" as const,
  orderDirection: "ASC" as const,
};

export function AdminMastersClient() {
  const t = useTranslations("AdminMasters");
  const tCommon = useTranslations("Common");
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState<string | null>(null);
  const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);
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

  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // In-flight guard MUST be useRef<boolean> — see
  // docs/pagination/intersection-observer-in-flight-guard.md.
  const fetchingRef = useRef(false);

  // Debounce search 300ms after the last keystroke.
  useEffect(() => {
    const timer = setTimeout(() => setSearchQuery(searchInput.trim() || null), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: searchQuery is an intentional trigger; the effect resets derived IO state, not searchQuery itself.
  useEffect(() => {
    fetchingRef.current = false;
    setFetchMoreError(null);
  }, [searchQuery]);

  const {
    data,
    fetchMore,
    loading,
    networkStatus,
    error: queryError,
    refetch,
  } = useQuery(AdminMastersQuery, {
    variables: { ...BASE_VARS, search: searchQuery },
    fetchPolicy: "cache-first",
    notifyOnNetworkStatusChange: true,
  });

  const queryErrorKind = classifyQueryError(queryError);
  const queryBannerError = queryErrorKind?.kind === "banner" ? queryErrorKind.message : undefined;

  const connection = data?.adminMasters;
  const edges: Edge[] = connection?.edges ?? [];
  const hasNextPage = connection?.pageInfo.hasNextPage ?? false;
  const endCursor = connection?.pageInfo.endCursor ?? null;
  const totalCount = connection?.totalCount ?? 0;

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

  const fetchNextPage = useCallback(
    ({ hasNextPage, endCursor, searchQuery }: FetchNextPageInput) => {
      if (fetchingRef.current || !hasNextPage) return;
      fetchingRef.current = true;
      fetchMore({
        variables: { ...BASE_VARS, after: endCursor, search: searchQuery },
        updateQuery: (prev, { fetchMoreResult }) => {
          if (!fetchMoreResult) return prev;
          return {
            adminMasters: {
              ...fetchMoreResult.adminMasters,
              edges: [...prev.adminMasters.edges, ...fetchMoreResult.adminMasters.edges],
            },
          };
        },
      })
        .then(() => setFetchMoreError(null))
        .catch((err) => {
          console.warn("[admin-masters] fetchMore failed", {
            name: err instanceof Error ? err.name : "unknown",
            searchQuery,
            endCursor,
          });
          setFetchMoreError(getBackendErrorBanner(err) ?? t("fetchMoreFailed"));
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
    if (!hasNextPage) return;
    if (fetchMoreError != null) return;
    const node = sentinelRef.current;
    if (!node) return;
    const observer = new IntersectionObserver((entries) => {
      if (!entries[0]?.isIntersecting || fetchingRef.current) return;
      requestNextPageFromObserver();
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [hasNextPage, fetchMoreError]);

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
          codes.includes("FORBIDDEN")
            ? t("forbidden")
            : codes.includes("UNAUTHENTICATED")
              ? t("unauthenticated")
              : t("unexpectedError"),
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
          codes.includes("FORBIDDEN")
            ? t("forbidden")
            : codes.includes("UNAUTHENTICATED")
              ? t("unauthenticated")
              : t("unexpectedError"),
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
          codes.includes("FORBIDDEN")
            ? t("forbidden")
            : codes.includes("UNAUTHENTICATED")
              ? t("unauthenticated")
              : t("deleteMasterFailed"),
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
          codes.includes("FORBIDDEN")
            ? t("forbidden")
            : codes.includes("UNAUTHENTICATED")
              ? t("unauthenticated")
              : t("unexpectedError"),
        );
      }
    },
    [runPublish, runUnpublish, t],
  );

  const fetchingMore = networkStatus === NetworkStatus.fetchMore || (loading && edges.length > 0);
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
            onClick={() => {
              setFetchMoreError(null);
              fetchNextPage({ hasNextPage, endCursor, searchQuery });
            }}
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
