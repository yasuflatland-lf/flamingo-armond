"use client";

import { NetworkStatus } from "@apollo/client";
import { useLazyQuery, useQuery } from "@apollo/client/react";
import Link from "next/link";
import { useCallback, useEffect, useEffectEvent, useMemo, useRef, useState } from "react";
import { useFragment } from "@/generated/fragment-masking";
import type { AdminUsersQuery as AdminUsersQueryResult } from "@/generated/graphql";
import {
  classifyQueryError,
  getBackendErrorBanner,
  type QueryErrorKind,
} from "@/lib/apollo/errors";
import type { FetchNextPageInput } from "@/lib/pagination/types";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import { AdminUserProfileSheet } from "./admin-user-profile-sheet";
import { type AdminUserListItem, AdminUserRow } from "./admin-user-row";
import {
  ADMIN_USERS_PAGE_SIZE,
  AdminUserProfileFieldsFragment,
  AdminRoleFieldsFragment,
  AdminRolesQuery,
  AdminUserFieldsFragment,
  AdminUserQuery,
  AdminUsersQuery,
} from "./queries";

type Connection = AdminUsersQueryResult["users"];
type Edge = Connection["edges"][number];

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
function formatEditUserBannerError(kind: QueryErrorKind | null): string | null {
  if (!kind) return null;
  switch (kind.kind) {
    case "forbidden":
      return "You do not have permission to edit this user.";
    case "unauthenticated":
      return "Your session has expired. Sign in again.";
    case "banner":
      return kind.message;
  }
}

export function AdminUsersClient() {
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState<string | null>(null);
  const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);
  const sheet = useSheetSearchParam();

  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // In-flight guard MUST be useRef<boolean>, not useState — see
  // docs/pagination/intersection-observer-in-flight-guard.md.
  const fetchingRef = useRef(false);

  // Debounce: update searchQuery 300ms after the last keystroke.
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearchQuery(searchInput.trim() || null);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // When the active search query changes, any in-flight fetchMore from the
  // previous search holds a stale cursor. Reset the IO guard and error state
  // immediately so the new query starts from a clean slate.
  // See docs/pagination/intersection-observer-in-flight-guard.md.
  // biome-ignore lint/correctness/useExhaustiveDependencies: searchQuery is an intentional trigger dependency; it is not referenced in the body because the effect resets derived IO state, not searchQuery itself.
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
  } = useQuery(AdminUsersQuery, {
    variables: { first: ADMIN_USERS_PAGE_SIZE, search: searchQuery },
    fetchPolicy: "cache-first",
    notifyOnNetworkStatusChange: true,
  });
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

  const connection = data?.users;
  const edges: Edge[] = connection?.edges ?? [];
  const hasNextPage = connection?.pageInfo.hasNextPage ?? false;
  const endCursor = connection?.pageInfo.endCursor ?? null;
  const totalCount = connection?.totalCount ?? 0;
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
  const editUserBannerError = formatEditUserBannerError(editUserErrorKind);

  useEffect(() => {
    if (!editUserId) return;
    void loadAdminUser({ variables: { id: editUserId } });
  }, [editUserId, loadAdminUser]);

  const reloadEditedUser = useCallback(() => {
    void refetch();
    if (!editUserId) return;
    void loadAdminUser({ variables: { id: editUserId } });
  }, [editUserId, loadAdminUser, refetch]);

  const fetchNextPage = useCallback(
    ({ hasNextPage, endCursor, searchQuery }: FetchNextPageInput) => {
      if (fetchingRef.current || !hasNextPage) return;

      fetchingRef.current = true;
      fetchMore({
        variables: {
          first: ADMIN_USERS_PAGE_SIZE,
          after: endCursor,
          search: searchQuery,
        },
        updateQuery: (prev, { fetchMoreResult }) => {
          if (!fetchMoreResult) return prev;
          return {
            users: {
              ...fetchMoreResult.users,
              edges: [...prev.users.edges, ...fetchMoreResult.users.edges],
            },
          };
        },
      })
        .then(() => {
          setFetchMoreError(null);
        })
        .catch((err) => {
          // Structured warn for operator triage: name + request context only.
          // err.message is omitted — backend messages may carry user-authored content.
          // See docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md.
          console.warn("[admin-users] fetchMore failed", {
            name: err instanceof Error ? err.name : "unknown",
            searchQuery,
            endCursor,
          });
          const banner =
            getBackendErrorBanner(err) ?? "Could not load more users. Please try again.";
          setFetchMoreError(banner);
        })
        .finally(() => {
          fetchingRef.current = false;
        });
    },
    [fetchMore],
  );

  const requestNextPageFromObserver = useEffectEvent(() => {
    fetchNextPage({ hasNextPage, endCursor, searchQuery });
  });

  useEffect(() => {
    if (!hasNextPage) return;
    // Halt the observer loop while a previous fetch failed; user must click Retry to resume.
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

  const fetchingMore = networkStatus === NetworkStatus.fetchMore || (loading && edges.length > 0);
  const initialLoading = loading && edges.length === 0 && networkStatus !== NetworkStatus.fetchMore;

  return (
    <main className="p-8">
      <div className="mb-6 flex items-center gap-4">
        <h1 className="text-2xl font-semibold">Users</h1>
        <span className="text-sm text-muted-foreground">({totalCount})</span>
      </div>

      {/* Search input */}
      <div className="mb-6">
        <input
          type="search"
          placeholder="Search users..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label="Search users"
        />
      </div>

      {/* FORBIDDEN error banner — no Retry since re-issuing the query would fail again */}
      {queryErrorKind?.kind === "forbidden" && (
        <div
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-users-query-error"
        >
          You do not have permission to view this page.
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
          <span>Your session has expired. </span>
          <Link href="/login" className="underline">
            Please sign in again.
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
            Retry
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

      {/* Loading state */}
      {initialLoading && (
        <p className="text-sm text-muted-foreground" data-testid="admin-users-loading">
          Loading...
        </p>
      )}

      {/* Empty state */}
      {!initialLoading && !queryErrorKind && edges.length === 0 && (
        <p className="text-sm text-muted-foreground" data-testid="admin-users-empty">
          No users found.
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
            onClick={() => {
              setFetchMoreError(null);
              fetchNextPage({ hasNextPage, endCursor, searchQuery });
            }}
          >
            Retry
          </button>
        </div>
      )}

      {/* Loading more indicator */}
      {!fetchMoreError && fetchingMore && hasNextPage && (
        <p
          className="mt-3 text-center text-xs text-muted-foreground"
          data-testid="admin-users-loading-more"
        >
          Loading more users...
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
      />
    </main>
  );
}
