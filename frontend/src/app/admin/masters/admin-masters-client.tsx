"use client";

import { NetworkStatus } from "@apollo/client";
import { Plus } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useMemo, useState } from "react";
import { toast } from "sonner";
import { AdminQueryErrorBanner } from "@/components/admin/admin-query-error-banner";
import { ConnectionListFooter } from "@/components/layout/connection-list-footer";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { SearchTakeoverBar } from "@/components/search/search-takeover-bar";
import { Button } from "@/components/ui/button";
import { FormSheet } from "@/components/ui/form-sheet";
import { Input } from "@/components/ui/input";
import type {
  AdminMastersQuery as AdminMastersQueryResult,
  AdminMastersQueryVariables,
} from "@/generated/graphql";
import { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import { classifyQueryError, getBackendErrorBanner } from "@/lib/apollo/errors";
import { EMPTY_PAGE_INFO } from "@/lib/pagination/empty-page-info";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import { AdminMasterForm, type MasterFormValues } from "./admin-master-form";
import { AdminMasterRow } from "./admin-master-row";
import { AdminMastersSkeleton } from "./admin-masters-skeleton";
import { ADMIN_MASTERS_BASE_VARS, AdminMastersQuery } from "./queries";
import { useMasterMutations } from "./use-master-mutations";

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
  const router = useRouter();
  const search = useHeaderTakeoverSearch();
  const searchQuery = search.query;
  const [createDirty, setCreateDirty] = useState(false);
  const [createValidationError, setCreateValidationError] = useState<{
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

  const { createMaster, creating, resetCreate } = useMasterMutations();

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
          setCreateDirty(false);
          router.push(`/admin/masters/${outcome.id}/edit`);
          return;
        case "auth":
          toast.error(outcome.kind === "forbidden" ? t("forbidden") : t("unauthenticated"));
          return;
        default:
          toast.error(t("unexpectedError"));
      }
    },
    [createMaster, router, t],
  );

  const initialLoading = networkStatus === NetworkStatus.loading && edges.length === 0;

  if (initialLoading) return <AdminMastersSkeleton />;

  return (
    <>
      <SearchTakeoverBar
        open={search.searchOpen}
        value={search.input}
        onChange={search.setInput}
        onClear={search.clear}
        onClose={search.closeSearch}
        placeholder={t("searchPlaceholder")}
        ariaLabel={t("searchLabel")}
      />
      <ListingPageShell
        title={t("title")}
        count={totalCount}
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
        <div className="hidden md:block">
          <Input
            type="search"
            placeholder={t("searchPlaceholder")}
            value={search.input}
            onChange={(e) => search.setInput(e.target.value)}
            aria-label={t("searchLabel")}
          />
        </div>

        <AdminQueryErrorBanner
          kind={queryErrorKind}
          onRetry={refetch}
          testId="admin-masters-query-error"
          copy={{
            viewForbidden: t("viewForbidden"),
            sessionExpired: t("sessionExpired"),
            signInAgain: t("pleaseSignInAgain"),
            retry: tCommon("retry"),
          }}
        />

        {!initialLoading && !queryErrorKind && edges.length === 0 && (
          <p className="text-sm text-muted-foreground" data-testid="admin-masters-empty">
            {t("noMastersFound")}
          </p>
        )}

        {edges.length > 0 && (
          <ul className="space-y-3" data-testid="admin-masters-list">
            {edges.map((edge) => (
              <AdminMasterRow key={edge.cursor} master={edge.node} />
            ))}
          </ul>
        )}

        <ConnectionListFooter
          sentinelRef={sentinelRef}
          fetchMoreError={fetchMoreError}
          onRetry={retryFetchMore}
          fetchingMore={fetchingMore}
          hasNextPage={hasNextPage}
          retryLabel={tCommon("retry")}
          loadingMoreLabel={t("loadingMore")}
          testIdPrefix="admin-masters"
        />

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
      </ListingPageShell>
    </>
  );
}
