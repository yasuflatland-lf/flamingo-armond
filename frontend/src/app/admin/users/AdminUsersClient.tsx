"use client";

import { useApolloClient, useQuery } from "@apollo/client/react";
import { redirect, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Button } from "@/components/ui/button";
import {
  type AdminRoleFieldsFragment as AdminRoleFieldsFragmentType,
  AdminRolesDocument,
  type AdminUserFieldsFragment as AdminUserFieldsFragmentType,
  AdminUsersDocument,
  type AdminUsersQuery,
} from "@/generated/graphql";
import { classifyQueryError } from "@/lib/apollo/errors";
import { ADMIN_USERS_DEFAULT_VARS, ADMIN_USERS_PAGE_SIZE } from "./queries";
import { type AdminUserRow, getUsersColumns } from "./users-columns";
import { UsersTable } from "./users-table";
import { UsersToolbar } from "./users-toolbar";

type Connection = AdminUsersQuery["users"];
type Edge = Connection["edges"][number];

interface AdminUsersClientProps {
  /**
   * SSR-seeded connection for the default view (no search, no role filter,
   * page 0). Written to the cache once on mount so the first useQuery pass
   * is a cache hit and the page renders without a client-side round-trip.
   * Null when the SSR seed failed (the page-level catch logs and falls
   * through to a client-side fetch).
   */
  initialConnection: Connection | null;
}

/**
 * Convert a masked Connection edge into the AdminUserRow shape consumed by
 * the DataTable. Fragment masking is compile-time only (see
 * `frontend/src/generated/fragment-masking.ts` — `useFragment` is a pure
 * type-cast at runtime), so a runtime `as` assertion is the established
 * pattern in this codebase. Reference:
 *   frontend/src/app/admin/users/[id]/edit/AdminUserEditClient.tsx
 *   ("fragment masking is compile-time only; at runtime the shape is the
 *   plain object").
 */
function edgeToRow(edge: Edge): AdminUserRow {
  // Fragment masking is compile-time only at runtime — see docs/frontend.md.
  const node = edge.node as unknown as AdminUserFieldsFragmentType & {
    roles: AdminRoleFieldsFragmentType[];
  };
  return {
    id: node.id,
    displayName: node.displayName ?? null,
    avatarUrl: node.avatarUrl ?? null,
    lastActive: node.lastActive ?? null,
    roles: node.roles.map((r) => ({ id: r.id, name: r.name })),
  };
}

export function AdminUsersClient({ initialConnection }: AdminUsersClientProps) {
  const apollo = useApolloClient();
  const router = useRouter();
  const searchParams = useSearchParams();

  // ------------------------------------------------------------------
  // State
  // ------------------------------------------------------------------
  // searchInput is the raw text the user is typing; searchQuery is the
  // debounced value that drives the GraphQL request.
  const [searchInput, setSearchInput] = useState(() => searchParams.get("search") ?? "");
  const [searchQuery, setSearchQuery] = useState<string | null>(
    () => searchParams.get("search")?.trim() || null,
  );
  const [roleFilter, setRoleFilter] = useState<string | null>(
    () => searchParams.get("roleId") || null,
  );
  // Page index is intentionally not URL-synced; cursor walks make non-zero
  // deep links unsupportable. Search and roleId remain URL-synced.
  const [pageIndex, setPageIndex] = useState<number>(0);
  const [pageSize, setPageSize] = useState<number>(ADMIN_USERS_PAGE_SIZE);
  // Map page index -> the `after` cursor that produces that page (page 0 = null).
  // Discrete pagination over a Relay Connection requires a cursor walk: when
  // navigating to page N for the first time, we walk forward issuing fetchMore
  // for each missing cursor and stash each resulting endCursor. For backward
  // navigation, the cursor is already cached.
  // Cursor walks N times for "goto page N" first visits — acceptable because
  // the admin users count is bounded (operator workflow, not a high-traffic
  // listing) and cached cursors short-circuit subsequent revisits.
  const [cursorByPage, setCursorByPage] = useState<Map<number, string | null>>(
    () => new Map([[0, null]]),
  );
  const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);
  // Track whether the current fetchMoreError is a FORBIDDEN error. Retry would
  // loop forever on a session-revoked role, so the Retry button must be
  // suppressed in that case.
  const [fetchMoreErrorIsForbidden, setFetchMoreErrorIsForbidden] = useState(false);

  // Strict Mode / re-render safe in-flight guard for fetchMore. Required to
  // prevent overlapping cursor walks. See .claude/rules/pagination.md.
  const fetchingRef = useRef(false);
  // One-shot SSR seed write into the Apollo cache.
  const seededRef = useRef(false);

  // ------------------------------------------------------------------
  // SSR seed: write initialConnection into the cache once on mount with
  // ADMIN_USERS_DEFAULT_VARS as the cache key. useQuery runs during the
  // first render before this effect fires, so the initial pass still
  // dispatches a fetch — but the seed populates the cache before paint
  // so the rendered tree shows server-fetched data without a network
  // round-trip blocking the user. Non-default URLs (search, role)
  // miss the seed; they fall through to a fetch.
  // ------------------------------------------------------------------
  useEffect(() => {
    if (seededRef.current || initialConnection == null) return;
    seededRef.current = true;
    apollo.writeQuery({
      query: AdminUsersDocument,
      variables: ADMIN_USERS_DEFAULT_VARS,
      data: { users: initialConnection },
    });
  }, [apollo, initialConnection]);

  // ------------------------------------------------------------------
  // Debounced search: update searchQuery 300ms after the last keystroke.
  // ------------------------------------------------------------------
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearchQuery(searchInput.trim() || null);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // ------------------------------------------------------------------
  // Filter change reset: when searchQuery or roleFilter changes, the prior
  // cursor walk is invalidated. Reset pageIndex, the cursor map, the in-flight
  // guard, and any stale error banner.
  // See .claude/rules/pagination.md § "Reset the guard ref AND the error banner
  // when the active filter changes".
  // ------------------------------------------------------------------
  // biome-ignore lint/correctness/useExhaustiveDependencies: searchQuery and roleFilter are intentional trigger dependencies; the body resets derived pagination state, not the trigger values themselves.
  useEffect(() => {
    fetchingRef.current = false;
    setFetchMoreError(null);
    setFetchMoreErrorIsForbidden(false);
    setPageIndex(0);
    setCursorByPage(new Map([[0, null]]));
  }, [searchQuery, roleFilter]);

  // ------------------------------------------------------------------
  // URL sync: reflect searchQuery / roleFilter into ?search&roleId.
  // Page index is intentionally NOT synced — cursor walks make non-zero deep
  // links unsupportable (we'd need cursors we never fetched). Build via
  // URLSearchParams (never string-interpolated).
  // ------------------------------------------------------------------
  useEffect(() => {
    const params = new URLSearchParams();
    if (searchQuery !== null) params.set("search", searchQuery);
    if (roleFilter !== null) params.set("roleId", roleFilter);
    const qs = params.toString();
    const target = qs.length > 0 ? `/admin/users?${qs}` : "/admin/users";
    router.replace(target, { scroll: false });
  }, [router, searchQuery, roleFilter]);

  // ------------------------------------------------------------------
  // Apollo wiring
  // ------------------------------------------------------------------
  // Variables for the active page. When pageIndex === 0 and there is no search
  // / role filter, this matches ADMIN_USERS_DEFAULT_VARS exactly so the cache
  // key aligns with the SSR seed. For non-default views we spread from the
  // default so any future-added variable in the schema lands in one place.
  const queryVariables = useMemo(() => {
    const after = cursorByPage.get(pageIndex) ?? null;
    return {
      ...ADMIN_USERS_DEFAULT_VARS,
      first: pageSize,
      search: searchQuery,
      roleId: roleFilter,
      after,
    };
  }, [cursorByPage, pageIndex, pageSize, searchQuery, roleFilter]);

  const {
    data,
    fetchMore,
    loading,
    error: queryError,
    refetch,
  } = useQuery(AdminUsersDocument, {
    variables: queryVariables,
    fetchPolicy: "cache-first",
    notifyOnNetworkStatusChange: true,
  });

  // Fetch the role list for the toolbar. Errors are non-fatal: render an empty
  // list and let the rest of the page work. Fragment masking is compile-time
  // only at runtime, so a single `as` cast unwraps the whole array.
  const rolesResult = useQuery(AdminRolesDocument, { fetchPolicy: "cache-first" });
  const availableRoles = useMemo(() => {
    const masked = (rolesResult.data?.roles ?? []) as unknown as AdminRoleFieldsFragmentType[];
    return masked.map((r) => ({ id: r.id, name: r.name }));
  }, [rolesResult.data]);

  // Surface an AdminRoles query failure to operator triage. The dropdown
  // silently degrades to empty (non-fatal for the rest of the page), but the
  // failure must be observable in logs. err.message is omitted because backend
  // GraphQL error messages may carry user-authored content (PII gate).
  // The dep array lists rolesResult.error as the trigger and the body reads
  // only that field, so the deps are intentionally exhaustive.
  useEffect(() => {
    if (rolesResult.error) {
      console.warn("[admin-users] AdminRoles query failed", {
        name: rolesResult.error.name,
      });
    }
  }, [rolesResult.error]);

  const queryErrorKind = classifyQueryError(queryError);

  // UNAUTHENTICATED post-mount: session expired mid-session. Redirect to "/"
  // where the app re-auths. (page.tsx blocks the initial load.)
  if (queryErrorKind?.kind === "unauthenticated") {
    redirect("/");
  }

  const queryBannerError = queryErrorKind?.kind === "banner" ? queryErrorKind.message : undefined;

  const connection = data?.users;
  const totalCount = connection?.totalCount ?? 0;
  const endCursor = connection?.pageInfo.endCursor ?? null;

  // Hydrate one AdminUserRow per masked edge. edgeToRow is a pure function so
  // useMemo memoisation is safe.
  const rows: AdminUserRow[] = useMemo(
    () => (connection?.edges ?? []).map(edgeToRow),
    [connection],
  );

  // ------------------------------------------------------------------
  // Page navigation: cursor walk to a target page index.
  //
  // The cursor model does not natively support "goto page N", so we walk
  // forward from the highest cached cursor: for each missing intermediate page
  // we fetchMore with the previous page's endCursor and stash the new endCursor
  // in cursorByPage. Backward navigation is a single state update (the cursor
  // is already cached). The fetchMore updateQuery REPLACES edges so the cache
  // reflects only the current page (not a concatenation across pages).
  // ------------------------------------------------------------------
  const handlePageChange = useCallback(
    (next: number) => {
      if (next < 0 || fetchingRef.current) return;
      // Backward navigation or current page: cursor is already cached.
      if (cursorByPage.has(next)) {
        setPageIndex(next);
        return;
      }
      // Forward navigation: walk one step at a time. The pagination control
      // calls this for `pageIndex + 1`, so the cursor we need is the endCursor
      // of the currently displayed page.
      if (endCursor === null) return;
      fetchingRef.current = true;
      fetchMore({
        variables: {
          ...ADMIN_USERS_DEFAULT_VARS,
          first: pageSize,
          search: searchQuery,
          roleId: roleFilter,
          after: endCursor,
        },
        updateQuery: (prev, { fetchMoreResult }) => fetchMoreResult ?? prev,
      })
        .then(() => {
          setCursorByPage((prev) => {
            const updated = new Map(prev);
            updated.set(next, endCursor);
            return updated;
          });
          setPageIndex(next);
          setFetchMoreError(null);
          setFetchMoreErrorIsForbidden(false);
        })
        .catch((err) => {
          const kind = classifyQueryError(err);
          if (kind?.kind === "unauthenticated") {
            redirect("/");
            return;
          }
          // Structured warn for operator triage. err.message is omitted because
          // backend messages may carry user-authored content.
          // See .claude/rules/frontend-typescript-conventions.md.
          console.warn("[admin-users] fetchMore failed", {
            name: err instanceof Error ? err.name : "unknown",
            pageIndex: next,
            searchQuery,
            roleId: roleFilter,
          });
          if (kind?.kind === "forbidden") {
            setFetchMoreError("You no longer have permission to load more users.");
            setFetchMoreErrorIsForbidden(true);
            return;
          }
          setFetchMoreError(kind?.message ?? "Could not load this page. Please try again.");
          setFetchMoreErrorIsForbidden(false);
        })
        .finally(() => {
          fetchingRef.current = false;
        });
    },
    [cursorByPage, endCursor, fetchMore, pageSize, roleFilter, searchQuery],
  );

  const columns = useMemo(() => getUsersColumns(), []);

  return (
    <ListingPageShell
      title="Users"
      description="Manage user accounts and role assignments."
      toolbar={
        <UsersToolbar
          searchInput={searchInput}
          onSearchInputChange={setSearchInput}
          roleFilterValue={roleFilter}
          onRoleFilterChange={setRoleFilter}
          availableRoles={availableRoles}
        />
      }
    >
      {/* FORBIDDEN — no Retry; re-issuing the query would fail again */}
      {queryErrorKind?.kind === "forbidden" && (
        <div
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-users-query-error"
        >
          You do not have permission to view this page.
        </div>
      )}

      {/* Generic query error with Retry */}
      {queryBannerError !== undefined && (
        <div
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-users-query-error"
        >
          <span>{queryBannerError}</span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="ml-3"
            onClick={() => refetch()}
          >
            Retry
          </Button>
        </div>
      )}

      {/* Page-fetch error with Retry — blocks further navigation while showing.
          Equivalent to the IO halt gate in the prior infinite-scroll design;
          see .claude/rules/pagination.md § "fetchMoreError != null halts the
          IO loop". */}
      {fetchMoreError !== null && (
        <div
          className="flex flex-col items-center gap-2 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="admin-users-fetch-more-error"
        >
          <span>{fetchMoreError}</span>
          {/* FORBIDDEN — no Retry; re-issuing the request would fail again */}
          {!fetchMoreErrorIsForbidden && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => {
                setFetchMoreError(null);
                handlePageChange(pageIndex + 1);
              }}
            >
              Retry
            </Button>
          )}
        </div>
      )}

      <UsersTable
        columns={columns}
        data={rows}
        pageIndex={pageIndex}
        pageSize={pageSize}
        totalCount={totalCount}
        onPageChange={handlePageChange}
        onPageSizeChange={setPageSize}
        isLoading={loading && rows.length === 0}
      />
    </ListingPageShell>
  );
}
