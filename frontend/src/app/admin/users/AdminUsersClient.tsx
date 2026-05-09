"use client";

import { NetworkStatus } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import Image from "next/image";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { useFragment } from "@/generated/fragment-masking";
import { AdminUsersDocument, type AdminUsersQuery } from "@/generated/graphql";
import { classifyQueryError, getBackendErrorBanner } from "@/lib/apollo/errors";
import { ADMIN_USERS_PAGE_SIZE, AdminRoleFieldsFragment, AdminUserFieldsFragment } from "./queries";

type Connection = AdminUsersQuery["users"];
type Edge = Connection["edges"][number];

function UserRow({ edge }: { edge: Edge }) {
  const user = useFragment(AdminUserFieldsFragment, edge.node);
  const roles = useFragment(AdminRoleFieldsFragment, edge.node.roles);

  return (
    <li
      key={user.id}
      className="flex rounded-md border border-border hover:bg-accent transition-colors"
      data-testid={`admin-user-row-${user.id}`}
    >
      <Link
        href={`/admin/users/${user.id}/edit`}
        aria-label={`Edit ${user.displayName ?? "user"}`}
        className="flex min-w-0 flex-1 items-start gap-4 px-4 py-3"
      >
        {/* Avatar */}
        {user.avatarUrl ? (
          <Image
            src={user.avatarUrl}
            alt={user.displayName ?? "User avatar"}
            width={40}
            height={40}
            className="h-10 w-10 shrink-0 rounded-full object-cover"
          />
        ) : (
          <div
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-muted text-sm font-medium text-muted-foreground"
            aria-hidden="true"
          >
            {(user.displayName ?? "?").charAt(0).toUpperCase()}
          </div>
        )}

        {/* Name, bio, roles */}
        <div className="min-w-0 flex-1 space-y-1">
          <p className="text-sm font-medium">
            {user.displayName ?? <span className="italic text-muted-foreground">No name</span>}
          </p>
          {user.bio && <p className="truncate text-sm text-muted-foreground">{user.bio}</p>}
          {roles.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {roles.map((role) => (
                <span
                  key={role.id}
                  className="inline-flex items-center rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
                >
                  {role.name}
                </span>
              ))}
            </div>
          )}
        </div>
      </Link>
    </li>
  );
}

export function AdminUsersClient() {
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState<string | null>(null);
  const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);

  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // docs/pagination/intersection-observer-in-flight-guard.md: in-flight guard MUST be useRef<boolean>, not useState.
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
  } = useQuery(AdminUsersDocument, {
    variables: { first: ADMIN_USERS_PAGE_SIZE, search: searchQuery },
    fetchPolicy: "cache-first",
    notifyOnNetworkStatusChange: true,
  });

  const queryErrorKind = classifyQueryError(queryError);

  // UNAUTHENTICATED post-mount means the session expired while the page was
  // open. The server-side gate in page.tsx + the admin layout already block
  // the initial load (which redirects to "/"), so this only fires mid-session.
  // Render a degraded banner pointing to /login rather than calling
  // `redirect()` from a client component — see
  // .claude/rules/frontend-rsc-error-handling.md.
  const queryBannerError = queryErrorKind?.kind === "banner" ? queryErrorKind.message : undefined;

  const connection = data?.users;
  const edges: Edge[] = connection?.edges ?? [];
  const hasNextPage = connection?.pageInfo.hasNextPage ?? false;
  const endCursor = connection?.pageInfo.endCursor ?? null;
  const totalCount = connection?.totalCount ?? 0;

  // Mirror state into refs so requestNextPage can stay identity-stable across
  // cursor / search / hasNextPage changes. Otherwise the IO observer effect
  // (which depends on requestNextPage) disconnects + reconnects on every page.
  const endCursorRef = useRef(endCursor);
  const searchQueryRef = useRef(searchQuery);
  const hasNextPageRef = useRef(hasNextPage);
  useEffect(() => {
    endCursorRef.current = endCursor;
  }, [endCursor]);
  useEffect(() => {
    searchQueryRef.current = searchQuery;
  }, [searchQuery]);
  useEffect(() => {
    hasNextPageRef.current = hasNextPage;
  }, [hasNextPage]);

  const requestNextPage = useCallback(() => {
    if (fetchingRef.current) return;
    if (!hasNextPageRef.current) return;

    fetchingRef.current = true;
    fetchMore({
      variables: {
        first: ADMIN_USERS_PAGE_SIZE,
        after: endCursorRef.current,
        search: searchQueryRef.current,
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
        // See docs/frontend/typescript-conventions.md § "expect.objectContaining".
        console.warn("[admin-users] fetchMore failed", {
          name: err instanceof Error ? err.name : "unknown",
          searchQuery: searchQueryRef.current,
          endCursor: endCursorRef.current ?? null,
        });
        const banner = getBackendErrorBanner(err) ?? "Could not load more users. Please try again.";
        setFetchMoreError(banner);
      })
      .finally(() => {
        fetchingRef.current = false;
      });
  }, [fetchMore]);

  useEffect(() => {
    if (!hasNextPage) return;
    // Halt the observer loop while a previous fetch failed; user must click Retry to resume.
    if (fetchMoreError != null) return;
    const node = sentinelRef.current;
    if (!node) return;

    const observer = new IntersectionObserver((entries) => {
      const entry = entries[0];
      if (!entry?.isIntersecting) return;
      if (fetchingRef.current) return;
      requestNextPage();
    });

    observer.observe(node);
    return () => observer.disconnect();
  }, [hasNextPage, fetchMoreError, requestNextPage]);

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

      {/* UNAUTHENTICATED mid-session banner — degraded UI pointing at /login. */}
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
            <UserRow key={edge.cursor} edge={edge} />
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
              requestNextPage();
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
    </main>
  );
}
