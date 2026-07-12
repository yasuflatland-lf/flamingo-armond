"use client";

import type { ReactNode, RefObject } from "react";
import { ConnectionListFooter } from "@/components/layout/connection-list-footer";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { SearchTakeoverBar } from "@/components/search/search-takeover-bar";
import type { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";

interface PaginatedPublicListScreenProps {
  /**
   * Search slice driving the mobile header-takeover bar. Mirrors
   * `AdminListSearch`'s `{ search, placeholder, ariaLabel }` triple: `search` is
   * the consuming client's `useHeaderTakeoverSearch` instance, and the copy is
   * caller-specific. The desktop box is passed separately via `desktopSearch`
   * because the two public screens keep their own margins / testids on it.
   */
  search: {
    search: ReturnType<typeof useHeaderTakeoverSearch>;
    placeholder: string;
    ariaLabel: string;
  };
  /**
   * Desktop-only search box, rendered into the `ListingPageShell` toolbar slot.
   * Each screen keeps its exact wrapper (cardgroups' `CardgroupsToolbar`,
   * catalog's `mb-2 hidden md:block` + `catalog-search` testid), so this stays a
   * per-screen node rather than being reconstructed inside the shell.
   */
  desktopSearch: ReactNode;
  /** Page title rendered in the `ListingPageShell` header. */
  title: ReactNode;
  /** Total count rendered as the neutral header pill. */
  count: number;
  /** Localized count-pill label (e.g. "14 total"). */
  countLabel: ReactNode;
  /** Optional CTA cluster on the header's trailing edge (cardgroups' "New" button). */
  primaryActions?: ReactNode;
  /** True on the very first load (`loading` with no edges yet, outside `fetchMore`). */
  initialLoading: boolean;
  /** Localized "loading…" copy for the first-load `<p>`. */
  loadingLabel: ReactNode;
  /** True when the connection has no edges; drives the empty / empty-search branch. */
  isEmpty: boolean;
  /** True when a non-empty search is active; picks empty-search over empty. */
  hasSearch: boolean;
  /**
   * Per-screen empty-state (no active search). Carries its own `data-testid`
   * and copy — cardgroups renders a CTA cluster, catalog a bare `<p>`.
   */
  emptyState: ReactNode;
  /** Per-screen no-match state (active search, zero results). Carries its own `data-testid`. */
  emptySearchState: ReactNode;
  /**
   * Footer slice spread straight into `<ConnectionListFooter>`. The shape matches
   * `ConnectionListFooterProps` minus `testIdPrefix`, which the shell supplies
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
  /**
   * testid namespace shared across the first-load line (`{prefix}-loading`) and
   * the footer sentinel/error/loading-more ids. The empty / empty-search nodes
   * carry their own ids, since those already differ structurally per screen.
   */
  testIdPrefix: string;
  /** The list `<ul>` and any screen-specific banners (e.g. a delete-error banner). */
  children: ReactNode;
}

/**
 * Shared scaffold for the public paginated list screens (cardgroups, catalog).
 * It owns the chrome both screens repeated near-identically — the mobile
 * header-takeover search bar, the desktop search toolbar slot, the
 * loading / empty / empty-search three-branch gate, and the infinite-scroll
 * footer — so a fix to any of those lands in one place.
 *
 * Each screen keeps its own `useConnectionPagination` call, its row rendering,
 * and its EXACT per-screen copy and behavior: the empty / empty-search nodes and
 * the list `<ul>` (plus any screen-specific banners / sheets) arrive as props /
 * `children`. Only the surrounding chrome moves here.
 *
 * The admin sibling is `PaginatedAdminListScreen`; the two are kept separate
 * because the admin screens carry a query-error banner and a single flat
 * empty-state, while the public screens carry the three-branch trio and no
 * query-error banner.
 */
export function PaginatedPublicListScreen({
  search,
  desktopSearch,
  title,
  count,
  countLabel,
  primaryActions,
  initialLoading,
  loadingLabel,
  isEmpty,
  hasSearch,
  emptyState,
  emptySearchState,
  footer,
  testIdPrefix,
  children,
}: PaginatedPublicListScreenProps) {
  const { search: searchInstance, placeholder, ariaLabel } = search;

  return (
    <>
      <SearchTakeoverBar
        open={searchInstance.searchOpen}
        value={searchInstance.input}
        onChange={searchInstance.setInput}
        onClear={searchInstance.clear}
        onClose={searchInstance.closeSearch}
        placeholder={placeholder}
        ariaLabel={ariaLabel}
      />
      <ListingPageShell
        title={title}
        count={count}
        countLabel={countLabel}
        primaryActions={primaryActions}
        toolbar={desktopSearch}
      >
        {initialLoading && (
          <p className="text-sm text-muted-foreground" data-testid={`${testIdPrefix}-loading`}>
            {loadingLabel}
          </p>
        )}

        {!initialLoading && isEmpty && !hasSearch && emptyState}

        {!initialLoading && isEmpty && hasSearch && emptySearchState}

        {children}

        <ConnectionListFooter {...footer} testIdPrefix={testIdPrefix} />
      </ListingPageShell>
    </>
  );
}
