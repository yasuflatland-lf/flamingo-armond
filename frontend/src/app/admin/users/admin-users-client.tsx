"use client";

import { NetworkStatus } from "@apollo/client";
import { useLazyQuery, useQuery } from "@apollo/client/react";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo } from "react";
import { toast } from "sonner";
import { PaginatedAdminListScreen } from "@/components/admin/paginated-admin-list-screen";
import { ErrorBanner } from "@/components/ui/error-banner";
import { useFragment } from "@/generated/fragment-masking";
import type {
  AdminUsersQuery as AdminUsersQueryResult,
  AdminUsersQueryVariables,
} from "@/generated/graphql";
import { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import {
  classifyQueryError,
  getBackendErrorBanner,
  type QueryErrorKind,
} from "@/lib/apollo/errors";
import { EMPTY_PAGE_INFO } from "@/lib/pagination/empty-page-info";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import { AdminUserProfileSheet } from "./admin-user-profile-sheet";
import { type AdminUserListItem, AdminUserRow } from "./admin-user-row";
import { AdminUsersSkeleton } from "./admin-users-skeleton";
import {
  ADMIN_USERS_PAGE_SIZE,
  AdminRoleFieldsFragment,
  AdminRolesQuery,
  AdminUserFieldsFragment,
  AdminUserProfileFieldsFragment,
  AdminUserQuery,
  AdminUsersQuery,
} from "./queries";
import { useAdminUserMutations } from "./use-admin-user-mutations";

type Connection = AdminUsersQueryResult["users"];
type Edge = Connection["edges"][number];
type PageInfo = Connection["pageInfo"];

// Render fallback for useConnectionPagination before the first query resolves.
// Admin users has no SSR seed, so this is the initial render value; it keeps the
// empty-edges shape the inline implementation used (`?? []`).
const USERS_INITIAL = {
  edges: [] as Edge[],
  pageInfo: EMPTY_PAGE_INFO,
  totalCount: 0,
};

// Concatenate the next page's edges onto the cached users connection.
function mergeUsersConnection(
  prev: AdminUsersQueryResult,
  more: AdminUsersQueryResult,
): AdminUsersQueryResult {
  return {
    users: {
      ...more.users,
      edges: [...prev.users.edges, ...more.users.edges],
    },
  };
}

function UserRowFromEdge({ edge, onEdit }: { edge: Edge; onEdit: (id: string) => void }) {
  const user = useFragment(AdminUserFieldsFragment, edge.node);
  const roles = useFragment(AdminRoleFieldsFragment, edge.node.roles);
  const rowUser: AdminUserListItem = {
    id: user.id,
    version: 0,
    displayName: user.displayName ?? null,
    bio: user.bio ?? null,
    avatarUrl: user.avatarUrl ?? null,
    lastSignInAt: user.lastSignInAt ?? null,
    roles: roles.map((role) => ({ id: role.id, name: role.name })),
  };

  return <AdminUserRow user={rowUser} onEdit={onEdit} />;
}

// Flatten the three-branch `classifyQueryError` result into a single banner
// string for the edit sheet. Returns null when there is no error.
function formatEditUserBannerError(
  kind: QueryErrorKind | null,
  editUserForbidden: string,
  unauthenticated: string,
): string | null {
  if (!kind) return null;
  switch (kind.kind) {
    case "forbidden":
      return editUserForbidden;
    case "unauthenticated":
      return unauthenticated;
    case "banner":
      return kind.message;
  }
}

export function AdminUsersClient() {
  const t = useTranslations("Admin");
  const tCommon = useTranslations("Common");
  const tNav = useTranslations("Nav");
  const search = useHeaderTakeoverSearch();
  const searchQuery = search.query;
  const sheet = useSheetSearchParam();

  // The cache key is { first, search }; memoize on searchQuery so the hook's
  // useQuery does not re-subscribe on unrelated re-renders.
  const queryVariables = useMemo<AdminUsersQueryVariables>(
    () => ({ first: ADMIN_USERS_PAGE_SIZE, search: searchQuery }),
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
  } = useConnectionPagination<AdminUsersQueryResult, AdminUsersQueryVariables, Edge, PageInfo>({
    document: AdminUsersQuery,
    variables: queryVariables,
    searchQuery,
    selectConnection: (data) => data?.users,
    buildFetchMoreVariables: (after, search) => ({
      first: ADMIN_USERS_PAGE_SIZE,
      after,
      search,
    }),
    mergeConnection: mergeUsersConnection,
    initial: USERS_INITIAL,
    resolveFetchMoreError: (err) => getBackendErrorBanner(err) ?? t("fetchMoreFailed"),
    logScope: "[admin-users]",
  });
  const hasNextPage = pageInfo.hasNextPage;

  const { data: rolesData, error: rolesError } = useQuery(AdminRolesQuery, {
    fetchPolicy: "cache-first",
  });
  const [
    loadAdminUser,
    {
      data: editUserData,
      loading: editUserLoading,
      error: editUserError,
      called: editUserCalled,
      variables: editUserVariables,
    },
  ] = useLazyQuery(AdminUserQuery, {
    fetchPolicy: "network-only",
    notifyOnNetworkStatusChange: true,
  });

  // Fan out into three render branches: forbidden, unauthenticated, banner.
  // See `classifyQueryError` for the source of the discriminated kinds.
  const queryErrorKind = classifyQueryError(queryError);

  const allRoles = useFragment(AdminRoleFieldsFragment, rolesData?.roles ?? []);
  const roleOptions = useMemo(
    () => allRoles.map((role) => ({ id: role.id, name: role.name })),
    [allRoles],
  );
  const rolesBannerError = getBackendErrorBanner(rolesError);

  const editUserId = sheet.state.mode === "edit" ? sheet.state.id : null;
  const editUserFields = useFragment(
    AdminUserProfileFieldsFragment,
    editUserData?.adminUser ?? null,
  );
  const editUserRoles = useFragment(AdminRoleFieldsFragment, editUserData?.adminUser?.roles ?? []);
  const editUser: AdminUserListItem | null = editUserFields
    ? {
        id: editUserFields.id,
        version: editUserFields.version,
        displayName: editUserFields.displayName ?? null,
        bio: editUserFields.bio ?? null,
        avatarUrl: editUserFields.avatarUrl ?? null,
        // The edit sheet does not display last sign-in; the list path is the
        // only consumer of AdminUserListItem.lastSignInAt.
        lastSignInAt: null,
        roles: editUserRoles.map((role) => ({ id: role.id, name: role.name })),
      }
    : null;
  const sheetUser = editUser?.id === editUserId ? editUser : null;
  const editUserResultMatchesSheet = editUserId !== null && editUserVariables?.id === editUserId;
  const editUserErrorKind = classifyQueryError(editUserError);
  const editUserBannerError = formatEditUserBannerError(
    editUserErrorKind,
    t("editUserForbidden"),
    t("unauthenticated"),
  );

  useEffect(() => {
    if (!editUserId) return;
    void loadAdminUser({ variables: { id: editUserId } });
  }, [editUserId, loadAdminUser]);

  const reloadEditedUser = useCallback(() => {
    void refetch();
    if (!editUserId) return;
    void loadAdminUser({ variables: { id: editUserId } });
  }, [editUserId, loadAdminUser, refetch]);

  const { deleteUser } = useAdminUserMutations();

  // Deletes a user (mutation + cache eviction live in the hook), then toasts and
  // closes the sheet. Re-throws on failure so AdminUserProfileSheet's danger zone
  // can render the reason.
  const handleDeleteUser = useCallback(
    async (id: string) => {
      await deleteUser(id);
      toast.success(t("deleteUserSuccess"));
      sheet.close();
    },
    [deleteUser, t, sheet],
  );

  // Show the full-page skeleton only on the very first load (NetworkStatus.loading = 1).
  // Refetch and setVariables must not re-trigger the skeleton — a refetch mid-session
  // (e.g. after ConcurrentUpdateError) would unmount the open sheet and lose any banner.
  const initialLoading = networkStatus === NetworkStatus.loading && edges.length === 0;

  return (
    <PaginatedAdminListScreen
      search={{
        search,
        placeholder: t("searchPlaceholder"),
        ariaLabel: t("searchLabel"),
      }}
      title={tNav("users")}
      count={totalCount}
      countLabel={tCommon("totalCount", { count: totalCount })}
      queryErrorKind={queryErrorKind}
      errorCopy={{
        viewForbidden: t("viewForbidden"),
        sessionExpired: t("sessionExpired"),
        signInAgain: t("pleaseSignInAgain"),
        retry: tCommon("retry"),
      }}
      onRetry={refetch}
      queryErrorClassName="mb-4"
      isEmpty={edges.length === 0}
      emptyLabel={t("noUsersFound")}
      footer={{
        sentinelRef,
        fetchMoreError,
        onRetry: retryFetchMore,
        fetchingMore,
        hasNextPage,
        retryLabel: tCommon("retry"),
        loadingMoreLabel: t("loadingMore"),
      }}
      initialLoading={initialLoading}
      skeleton={<AdminUsersSkeleton />}
      testIdPrefix="admin-users"
    >
      {rolesBannerError && (
        <ErrorBanner className="mb-4" data-testid="admin-users-roles-error">
          {rolesBannerError}
        </ErrorBanner>
      )}

      {/* User list */}
      {edges.length > 0 && (
        <ul className="space-y-3" data-testid="admin-users-list">
          {edges.map((edge) => (
            <UserRowFromEdge
              key={edge.cursor}
              edge={edge}
              onEdit={(id) => sheet.open({ mode: "edit", id })}
            />
          ))}
        </ul>
      )}

      <AdminUserProfileSheet
        open={editUserId !== null}
        user={sheetUser}
        loading={
          editUserLoading ||
          (editUserId !== null && (!editUserCalled || !editUserResultMatchesSheet))
        }
        allRoles={roleOptions}
        queryError={editUserBannerError}
        onDismiss={() => sheet.close()}
        onSaved={() => sheet.close({ refresh: true })}
        onReloadRequested={reloadEditedUser}
        onDelete={handleDeleteUser}
      />
    </PaginatedAdminListScreen>
  );
}
