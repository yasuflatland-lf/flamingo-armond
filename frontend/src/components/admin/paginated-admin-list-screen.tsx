"use client";

import type { ReactNode, RefObject } from "react";
import { AdminListSearch } from "@/components/admin/admin-list-search";
import { AdminQueryErrorBanner } from "@/components/admin/admin-query-error-banner";
import { ConnectionListFooter } from "@/components/layout/connection-list-footer";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import type { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import type { QueryErrorKind } from "@/lib/apollo/errors";

interface PaginatedAdminListScreenProps {
  /**
   * Search slice spread straight into `<AdminListSearch>`. The shape matches
   * `AdminListSearchProps` so the scaffold can `{...search}` it without restating
   * the field names.
   */
  search: {
    search: ReturnType<typeof useHeaderTakeoverSearch>;
    placeholder: string;
    ariaLabel: string;
  };
  /** Page title rendered in the `ListingPageShell` header. */
  title: ReactNode;
  /** Total count rendered as the neutral header pill. */
  count: number;
  /** Localized count-pill label (e.g. "14 total"). */
  countLabel: ReactNode;
  /** Optional CTA cluster on the header's trailing edge (e.g. the masters "New" button). */
  primaryActions?: ReactNode;
  /** Discriminated query-error kind; null while healthy. */
  queryErrorKind: QueryErrorKind | null;
  /** Resolved copy for the three-branch query-error banner. */
  errorCopy: {
    viewForbidden: string;
    sessionExpired: string;
    signInAgain: string;
    retry: string;
  };
  /** Re-issues the list query (the hook's `refetch`) for the banner Retry. */
  onRetry: () => void;
  /** Optional class merge for the query-error banner (the users screen passes "mb-4"). */
  queryErrorClassName?: string;
  /** True when the connection has no edges; drives the empty-state. */
  isEmpty: boolean;
  /** Localized empty-state copy. */
  emptyLabel: ReactNode;
  /**
   * Footer slice spread straight into `<ConnectionListFooter>`. The shape matches
   * `ConnectionListFooterProps` minus `testIdPrefix`, which the scaffold supplies
   * from its own `testIdPrefix`.
   */
  footer: {
    sentinelRef: RefObject<HTMLDivElement | null>;
    fetchMoreError: string | null;
    onRetry: () => void;
    fetchingMore: boolean;
    hasNextPage: boolean;
    retryLabel: string;
    loadingMoreLabel: string;
  };
  /** True on the very first load (`NetworkStatus.loading` with no edges yet). */
  loading: boolean;
  /** Full-page skeleton rendered while `loading`. */
  skeleton: ReactNode;
  /**
   * testid namespace shared across the query-error banner (`{prefix}-query-error`),
   * the empty-state (`{prefix}-empty`), and the footer sentinel/error/loading ids.
   */
  testIdPrefix: string;
  /** The list `<ul>`, the screen's sheet(s), and any screen-specific banners. */
  children: ReactNode;
}

/**
 * Shared scaffold for the paginated admin list screens (users, masters). It owns
 * the chrome both screens repeated near-identically — the search controls, the
 * count pill, the query-error banner, the empty/skeleton gate, and the
 * infinite-scroll footer — so a fix to any of those lands in one place.
 *
 * Each screen keeps its own `useConnectionPagination` call, merge fn, row
 * component, and sheet(s); only the chrome moves here. The screen-specific list
 * `<ul>` and sheet(s) arrive as `children`. Roles is intentionally not a consumer
 * (it has no pagination/search/connection).
 *
 * The skeleton gate lives here too: a mid-session refetch must not re-trigger it
 * (that would unmount an open sheet and lose any banner), so the caller passes the
 * already-computed `loading` flag rather than a raw network status.
 */
export function PaginatedAdminListScreen({
  search,
  title,
  count,
  countLabel,
  primaryActions,
  queryErrorKind,
  errorCopy,
  onRetry,
  queryErrorClassName,
  isEmpty,
  emptyLabel,
  footer,
  loading,
  skeleton,
  testIdPrefix,
  children,
}: PaginatedAdminListScreenProps) {
  if (loading) return <>{skeleton}</>;

  return (
    <ListingPageShell
      title={title}
      count={count}
      countLabel={countLabel}
      primaryActions={primaryActions}
      toolbar={<AdminListSearch {...search} />}
    >
      {/*
        Query-error banner. UNAUTHENTICATED here means the session expired
        mid-page: the server-side gate in page.tsx + the admin layout block the
        initial load (redirecting to "/"), so the degraded /login banner only
        fires post-mount. FORBIDDEN renders without Retry since re-issuing the
        same query would fail again. See .claude/rules/frontend-rsc-error-handling.md.
      */}
      <AdminQueryErrorBanner
        kind={queryErrorKind}
        onRetry={onRetry}
        testId={`${testIdPrefix}-query-error`}
        className={queryErrorClassName}
        copy={errorCopy}
      />

      {!queryErrorKind && isEmpty && (
        <p className="text-sm text-muted-foreground" data-testid={`${testIdPrefix}-empty`}>
          {emptyLabel}
        </p>
      )}

      {children}

      <ConnectionListFooter {...footer} testIdPrefix={testIdPrefix} />
    </ListingPageShell>
  );
}
