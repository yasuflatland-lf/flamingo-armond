"use client";

import { NetworkStatus } from "@apollo/client";
import { useApolloClient, useLazyQuery, useMutation, useQuery } from "@apollo/client/react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { useFragment } from "@/generated/fragment-masking";
import type {
  AdminUsersQuery as AdminUsersQueryResult,
  AdminUsersQueryVariables,
} from "@/generated/graphql";
import {
  classifyQueryError,
  getBackendErrorBanner,
  type QueryErrorKind,
} from "@/lib/apollo/errors";
import { useConnectionPagination } from "@/lib/pagination/use-connection-pagination";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import { AdminUserProfileSheet } from "./admin-user-profile-sheet";
import { type AdminUserListItem, AdminUserRow } from "./admin-user-row";
import { AdminUsersSkeleton } from "./admin-users-skeleton";
import {
  ADMIN_USERS_PAGE_SIZE,
  AdminDeleteUserMutation,
  AdminRoleFieldsFragment,
  AdminRolesQuery,
  AdminUserFieldsFragment,
  AdminUserProfileFieldsFragment,
  AdminUserQuery,
  AdminUsersQuery,
} from "./queries";

type Connection = AdminUsersQueryResult["users"];
type Edge = Connection["edges"][number];
type PageInfo = Connection["pageInfo"];

const EMPTY_PAGE_INFO: PageInfo = {
  __typename: "PageInfo",
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: null,
  endCursor: null,
};

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

function UserRow({ edge, onEdit }: { edge: Edge; onEdit: (id: string) => void }) {
  const user = useFragment(AdminUserFieldsFragment, edge.node);
  const roles = useFragment(AdminRoleFieldsFragment, edge.node.roles);
  const rowUser: AdminUserListItem = {
    id: user.id,
    version: 0,
    displayName: user.displayName ?? null,
    bio: user.bio ?? null,
    avatarUrl: user.avatarUrl ?? null,
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
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState<string | null>(null);
  const sheet = useSheetSearchParam();

  // Debounce: update searchQuery 300ms after the last keystroke.
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearchQuery(searchInput.trim() || null);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

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
  const queryBannerError = queryErrorKind?.kind === "banner" ? queryErrorKind.message : undefined;

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

  const apolloClient = useApolloClient();
  const [runDeleteUser] = useMutation(AdminDeleteUserMutation);

  // Deletes a user, updates the cache, toasts, and closes the sheet. Rejects on
  // failure (e.g. FORBIDDEN) so the sheet's danger zone can render the reason.
  const handleDeleteUser = useCallback(
    async (id: string) => {
      const result = await runDeleteUser({ variables: { id } });
      if (!result.data?.adminDeleteUser) {
        throw new Error("adminDeleteUser returned false");
      }
      // Drop the deleted user from every cached `users` connection variant (each
      // search / pagination combo is a separate field entry). The edge cursor
      // equals the user id by schema contract, so filter on cursor and avoid
      // unmasking the node fragment. Then evict the now-orphaned entity.
      apolloClient.cache.modify({
        fields: {
          users(existing) {
            const connection = existing as {
              edges?: ReadonlyArray<{ cursor: string }>;
              totalCount?: number;
            };
            if (!connection.edges) return existing;
            const edges = connection.edges.filter((edge) => edge.cursor !== id);
            if (edges.length === connection.edges.length) return existing;
            return {
              ...connection,
              edges,
              totalCount: Math.max(0, (connection.totalCount ?? 0) - 1),
            };
          },
        },
      });
      const cacheId = apolloClient.cache.identify({ __typename: "User", id });
      if (cacheId) {
        apolloClient.cache.evict({ id: cacheId });
        apolloClient.cache.gc();
      }
      toast.success(t("deleteUserSuccess"));
      sheet.close();
    },
    [apolloClient, runDeleteUser, sheet, t],
  );

  // Show the full-page skeleton only on the very first load (NetworkStatus.loading = 1).
  // Refetch and setVariables must not re-trigger the skeleton — a refetch mid-session
  // (e.g. after ConcurrentUpdateError) would unmount the open sheet and lose any banner.
  const initialLoading = networkStatus === NetworkStatus.loading && edges.length === 0;

  if (initialLoading) {
    return <AdminUsersSkeleton />;
  }

  return (
    <main className="p-8">
      <div className="mb-6 flex items-center gap-4">
        <h1 className="text-2xl font-semibold">{tNav("users")}</h1>
        <span className="text-sm text-muted-foreground">({totalCount})</span>
      </div>

      {/* Search input */}
      <div className="mb-6">
        <input
          type="search"
          placeholder={t("searchPlaceholder")}
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={t("searchLabel")}
        />
      </div>

      {/* FORBIDDEN error banner — no Retry since re-issuing the query would fail again */}
      {queryErrorKind?.kind === "forbidden" && (
        <div
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-users-query-error"
        >
          {t("viewForbidden")}
        </div>
      )}

      {/*
        UNAUTHENTICATED post-mount means the session expired while the page was
        open. The server-side gate in page.tsx + the admin layout already block
        the initial load (which redirects to "/"), so this only fires mid-session.
        Render a degraded banner pointing to /login rather than calling
        `redirect()` from a client component — see
        .claude/rules/frontend-rsc-error-handling.md.
      */}
      {queryErrorKind?.kind === "unauthenticated" && (
        <div
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-users-query-error"
        >
          <span>{t("sessionExpired")}</span>{" "}
          <Link href="/login" className="underline">
            {t("pleaseSignInAgain")}
          </Link>
        </div>
      )}

      {/* Generic query error banner with Retry */}
      {queryBannerError && (
        <div
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-users-query-error"
        >
          <span>{queryBannerError}</span>
          <button type="button" className="ml-3 underline" onClick={() => refetch()}>
            {tCommon("retry")}
          </button>
        </div>
      )}

      {rolesBannerError && (
        <div
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-users-roles-error"
        >
          {rolesBannerError}
        </div>
      )}

      {/* Empty state */}
      {!initialLoading && !queryErrorKind && edges.length === 0 && (
        <p className="text-sm text-muted-foreground" data-testid="admin-users-empty">
          {t("noUsersFound")}
        </p>
      )}

      {/* User list */}
      {edges.length > 0 && (
        <ul className="space-y-3" data-testid="admin-users-list">
          {edges.map((edge) => (
            <UserRow
              key={edge.cursor}
              edge={edge}
              onEdit={(id) => sheet.open({ mode: "edit", id })}
            />
          ))}
        </ul>
      )}

      {/* Intersection sentinel for infinite scroll */}
      <div ref={sentinelRef} aria-hidden="true" data-testid="admin-users-sentinel" />

      {/* fetchMore error banner with Retry */}
      {fetchMoreError && (
        <div
          className="mt-3 flex flex-col items-center gap-2 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-users-fetch-more-error"
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

      {/* Loading more indicator */}
      {!fetchMoreError && fetchingMore && hasNextPage && (
        <p
          className="mt-3 text-center text-xs text-muted-foreground"
          data-testid="admin-users-loading-more"
        >
          {t("loadingMore")}
        </p>
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
    </main>
  );
}
