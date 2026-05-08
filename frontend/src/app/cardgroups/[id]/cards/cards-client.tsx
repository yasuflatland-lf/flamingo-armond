"use client";

import { NetworkStatus } from "@apollo/client";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { Plus, Search, Trash2, X } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  DeleteCardMutation,
  DeleteCardsMutation,
  UpdateCardMutation,
} from "@/app/cardgroups/queries";
import { CardForm } from "@/components/cardgroups/card-form";
import { SwipeableRow, type SwipeableRowHandle } from "@/components/cardgroups/swipeable-row";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { _pendingCount, flushPendingDeletes, scheduleDelete } from "@/lib/undo-delete";
import { cardsDefaultVars } from "./queries";

type Connection = CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"];
export type CardEdge = Connection["edges"][number];
export type CardConnectionPageInfo = Connection["pageInfo"];

type Props = {
  cardgroupId: string;
  initialEdges: CardEdge[];
  initialPageInfo: CardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Optional section header. When `undefined`, the default `<h2>Cards (n)</h2>`
   * is rendered. Pass a `ReactNode` to replace the header, `null` to suppress
   * it entirely, or a render function to access the live `totalCount` from
   * Apollo cache without spinning up a second `useQuery` in the parent.
   */
  sectionHeader?: ReactNode | ((args: { totalCount: number }) => ReactNode);
};

export function CardsClient({
  cardgroupId,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
  sectionHeader,
}: Props) {
  const apollo = useApolloClient();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);

  // Map of per-row SwipeableRow refs, keyed by card id. When the user taps a
  // different row to enter edit mode, we close any half-open row first via its
  // ref. Using a Map (not a ref to an object literal) avoids stale-closure
  // concerns: the same Map instance persists across renders.
  const rowRefs = useRef<Map<string, React.RefObject<SwipeableRowHandle | null>>>(new Map());
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  // Debounced search: searchInput is the immediate input value; searchQuery is
  // the value Apollo actually queries with, debounced 300ms after the last
  // keystroke. searchQuery null === no active filter.
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState<string | null>(null);

  // Per-row delete commit error banner. We surface this from the scheduleDelete
  // commit callback rather than relying on useMutation's `error` because the
  // useMutation hook's `error` clears between mutation calls; we want the
  // banner to persist until the user dismisses it implicitly via a successful
  // retry. See .claude/rules/pagination.md § "Do not reuse one mutation's
  // Apollo-managed error state for a sibling mutation's failure surface".
  const [deleteCommitError, setDeleteCommitError] = useState<string | null>(null);

  const pathname = usePathname();

  const toggleSelected = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const clearSelection = useCallback(() => {
    setSelectedIds(new Set());
  }, []);

  // Close any half-open SwipeableRow that is not the row being activated.
  // Called before setEditingId so the swipe reveal does not remain open while
  // the row beneath it switches to inline-edit mode.
  const closeOtherRows = useCallback((exceptCardId: string) => {
    for (const [id, ref] of rowRefs.current.entries()) {
      if (id !== exceptCardId) {
        ref.current?.close();
      }
    }
  }, []);

  // Debounce: update searchQuery 300ms after the last keystroke.
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearchQuery(searchInput.trim() || null);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // pagination.md: in-flight guard MUST be useRef<boolean>, not useState.
  const fetchingRef = useRef(false);

  // When the active search query changes, any in-flight fetchMore from the
  // previous search holds a stale cursor. Reset the IO guard and error state
  // immediately so the new query starts from a clean slate.
  // See .claude/rules/pagination.md § "Reset the guard ref AND the error banner".
  // biome-ignore lint/correctness/useExhaustiveDependencies: searchQuery is an intentional trigger dependency; it is not referenced in the body because the effect resets derived IO state, not searchQuery itself.
  useEffect(() => {
    fetchingRef.current = false;
    setFetchMoreError(null);
  }, [searchQuery]);

  // When searchQuery is null we use cardsDefaultVars verbatim so the cache key
  // matches the SSR seed exactly. For non-null searches we spread and override
  // `search`, keeping `cardgroupId`/`first` in sync with the default.
  const queryVariables: CardsByCardgroupConnectionQueryVariables = useMemo(
    () =>
      searchQuery === null
        ? cardsDefaultVars(cardgroupId)
        : { ...cardsDefaultVars(cardgroupId), search: searchQuery },
    [cardgroupId, searchQuery],
  );

  const {
    data,
    fetchMore,
    loading,
    networkStatus,
    error: queryError,
  } = useQuery(CardsByCardgroupConnectionDocument, {
    variables: queryVariables,
    fetchPolicy: "cache-first",
    notifyOnNetworkStatusChange: true,
  });

  const queryBannerError = getBackendErrorBanner(queryError);

  const connection = data?.cardsByCardgroupConnection;
  const edges = connection?.edges ?? initialEdges;
  const pageInfo = connection?.pageInfo ?? initialPageInfo;
  const totalCount = connection?.totalCount ?? initialTotalCount;

  // Mirror cursor-related page state into refs so requestNextPage can read them
  // without being listed as a dep. This prevents the IO observer effect from
  // disconnecting/reconnecting every time a page loads (which updates endCursor).
  // See .claude/rules/pagination.md § "IntersectionObserver in-flight guard via useRef<boolean>".
  const endCursorRef = useRef(pageInfo.endCursor);
  const hasNextPageRef = useRef(pageInfo.hasNextPage);
  const searchQueryRef = useRef(searchQuery);
  useEffect(() => {
    endCursorRef.current = pageInfo.endCursor;
  }, [pageInfo.endCursor]);
  useEffect(() => {
    hasNextPageRef.current = pageInfo.hasNextPage;
  }, [pageInfo.hasNextPage]);
  useEffect(() => {
    searchQueryRef.current = searchQuery;
  }, [searchQuery]);

  const requestNextPage = useCallback(() => {
    if (fetchingRef.current) return;
    if (!hasNextPageRef.current) return;

    fetchingRef.current = true;
    fetchMore({
      variables: {
        ...cardsDefaultVars(cardgroupId),
        after: endCursorRef.current,
        search: searchQueryRef.current,
      },
      updateQuery: (prev, { fetchMoreResult }) => {
        if (!fetchMoreResult) return prev;
        return {
          cardsByCardgroupConnection: {
            ...fetchMoreResult.cardsByCardgroupConnection,
            edges: [
              ...prev.cardsByCardgroupConnection.edges,
              ...fetchMoreResult.cardsByCardgroupConnection.edges,
            ],
          },
        };
      },
    })
      .then(() => {
        // Clear any previous fetchMore error on success so the observer can resume.
        setFetchMoreError(null);
      })
      .catch((err) => {
        // Structured warn for operator triage: name + request context only.
        // err.message is omitted — backend messages may carry user-authored content.
        // See .claude/rules/frontend-typescript-conventions.md § "expect.objectContaining".
        console.warn("[cards-client] fetchMore failed", {
          name: err instanceof Error ? err.name : "unknown",
          searchQuery: searchQueryRef.current ?? null,
          endCursor: endCursorRef.current ?? null,
        });
        const banner = getBackendErrorBanner(err) ?? "Could not load more cards. Please try again.";
        setFetchMoreError(banner);
      })
      .finally(() => {
        fetchingRef.current = false;
      });
  }, [cardgroupId, fetchMore]);

  useEffect(() => {
    if (!pageInfo.hasNextPage) return;
    // Stop the observer loop while a previous fetch failed; user must click Retry to resume.
    if (fetchMoreError != null) return;
    const node = sentinelRef.current;
    if (!node) return;

    const observer = new IntersectionObserver((entries) => {
      const entry = entries[0];
      if (!entry?.isIntersecting) return;
      if (fetchingRef.current) return;
      if (!hasNextPageRef.current) return;
      requestNextPage();
    });

    observer.observe(node);
    return () => observer.disconnect();
  }, [pageInfo.hasNextPage, fetchMoreError, requestNextPage]);

  // Update propagates automatically via Apollo cache normalization (Card has id).
  const [updateCard, { loading: updating, error: updateError }] = useMutation(UpdateCardMutation);

  // Per-row delete uses scheduleDelete (5s undo window). The mutation runs
  // imperatively from inside scheduleDelete's commit callback rather than via
  // an `update` callback on the hook — the optimistic remove happens BEFORE
  // the mutation fires (or never, if the user clicks Undo).
  const [deleteCardMutation] = useMutation(DeleteCardMutation);

  const [deleteCards, { error: bulkDeleteError, loading: bulkDeleting }] = useMutation(
    DeleteCardsMutation,
    {
      update(cache, { data: bulkData }, { variables: mutationVars }) {
        const ids = mutationVars?.ids as string[] | undefined;
        if (!ids) return;
        // Guard: when data is undefined (network failure) or deleteCards is null,
        // return early — do not evict, writeQuery, or gc.
        if (bulkData?.deleteCards == null) return;
        const deletedCount = bulkData.deleteCards;
        // Backend reports actual rows deleted; some ids may have been skipped (foreign-owned),
        // so deletedCount === 0 means nothing to mutate locally either.
        if (deletedCount === 0) return;

        // Use queryVariables (the same memo useQuery is keyed on) so the
        // readQuery / writeQuery pair targets the live cache entry under any
        // active search filter. cardsDefaultVars(cardgroupId) would mismatch
        // when searchQuery !== null and the optimistic remove would be lost.
        // See .claude/rules/pagination.md § "Variables shape MUST match
        // between SSR seed and client cache reads".
        const existing = cache.readQuery({
          query: CardsByCardgroupConnectionDocument,
          variables: queryVariables,
        });

        if (existing) {
          const filteredEdges = existing.cardsByCardgroupConnection.edges.filter(
            (edge) => !ids.includes(edge.node.id),
          );
          cache.writeQuery({
            query: CardsByCardgroupConnectionDocument,
            variables: queryVariables,
            data: {
              cardsByCardgroupConnection: {
                ...existing.cardsByCardgroupConnection,
                edges: filteredEdges,
                totalCount: Math.max(
                  0,
                  existing.cardsByCardgroupConnection.totalCount - deletedCount,
                ),
              },
            },
          });
        }

        for (const id of ids) {
          cache.evict({ id: cache.identify({ __typename: "Card", id }) });
        }
        cache.gc();
      },
    },
  );

  const bulkDeleteBannerError = getBackendErrorBanner(bulkDeleteError);

  async function handleBulkDelete() {
    const ids = Array.from(selectedIds);
    try {
      await deleteCards({ variables: { ids } });
      clearSelection();
    } catch (err) {
      // Structured log for operator triage: name + domain context only.
      // err.message is omitted — backend messages may carry user-authored content.
      // See .claude/rules/frontend-typescript-conventions.md § "expect.objectContaining".
      console.error("[CardsClient] bulk delete rejection", {
        name: err instanceof Error ? err.name : "unknown",
        cardgroupId,
        ids,
      });
    }
  }

  // Per-row delete: snapshot the current connection, optimistically drop the
  // edge, then schedule the real DELETE for 5 seconds via scheduleDelete. On
  // Undo (within 5s) the snapshot is restored. On commit failure, the snapshot
  // is restored and the banner surfaces the error.
  //
  // Uses `queryVariables` (the same memo useQuery is keyed on) for both the
  // snapshot read and the rollback writeQuery so the optimistic remove targets
  // the live cache entry under any active search filter.
  // See .claude/rules/pagination.md § "Variables shape MUST match between SSR
  // seed and client cache reads".
  const handleDeleteRow = useCallback(
    (cardId: string) => {
      const snapshot = apollo.readQuery({
        query: CardsByCardgroupConnectionDocument,
        variables: queryVariables,
      });

      // Optimistic step: drop the matching edge and decrement totalCount.
      if (snapshot) {
        const filteredEdges = snapshot.cardsByCardgroupConnection.edges.filter(
          (edge) => edge.node.id !== cardId,
        );
        apollo.writeQuery({
          query: CardsByCardgroupConnectionDocument,
          variables: queryVariables,
          data: {
            cardsByCardgroupConnection: {
              ...snapshot.cardsByCardgroupConnection,
              edges: filteredEdges,
              totalCount: Math.max(0, snapshot.cardsByCardgroupConnection.totalCount - 1),
            },
          },
        });
      }

      const optimisticRollback = () => {
        if (snapshot !== null) {
          apollo.writeQuery({
            query: CardsByCardgroupConnectionDocument,
            variables: queryVariables,
            data: snapshot,
          });
        }
        // Clear any prior failure banner on rollback so the UI returns to a clean state.
        setDeleteCommitError(null);
      };

      const commitDelete = async () => {
        // Clear any prior failure before retrying so the banner reflects this attempt.
        setDeleteCommitError(null);
        const result = await deleteCardMutation({ variables: { id: cardId } });
        // After a successful commit, evict the normalized entity so dangling
        // references are cleaned up. Mirrors pagination.md § "Connection delete".
        if (result.data?.deleteCard) {
          apollo.cache.evict({ id: apollo.cache.identify({ __typename: "Card", id: cardId }) });
          apollo.cache.gc();
        }
      };

      scheduleDelete({
        id: cardId,
        label: "Card deleted",
        optimisticRollback,
        commitDelete,
        onCommitFailed: (err) => {
          const banner = getBackendErrorBanner(err) ?? "Could not delete card. Please try again.";
          setDeleteCommitError(banner);
        },
      });
    },
    [apollo, deleteCardMutation, queryVariables],
  );

  // Flush pending deletes on pathname change so a user who navigates away
  // inside the 5-second undo window does not silently lose the DELETE request.
  //
  // Fire on the *change*, not in cleanup: cleanup callbacks run while React is
  // tearing the component down, so `void flushPendingDeletes()` returns a
  // Promise the runtime never awaits and the in-flight DELETE mutations get
  // dropped. Triggering inside the effect body — keyed on a previous-pathname
  // ref — runs while the component is still mounted and the async context is
  // alive long enough for Apollo to flush the mutations.
  const previousPathnameRef = useRef(pathname);
  useEffect(() => {
    if (previousPathnameRef.current !== pathname) {
      void flushPendingDeletes();
      previousPathnameRef.current = pathname;
    }
  }, [pathname]);

  // Browser-level navigation safety net for the same flush concern.
  //
  // NOTE: most browsers cancel pending fetch / XHR requests when `beforeunload`
  // fires, so the DELETE mutations triggered from here are NOT guaranteed to
  // reach the server — the user may navigate away before the request settles.
  // The console.warn surfaces the discard so operators can see in dev tools
  // that the pending DELETE may have been dropped. Switching to
  // `navigator.sendBeacon` would only work for endpoints that accept anonymous
  // POSTs; this app's GraphQL endpoint requires Authorization headers, which
  // sendBeacon cannot reliably attach. Document and accept the limitation.
  useEffect(() => {
    function onBeforeUnload() {
      if (_pendingCount() === 0) return;
      console.warn(
        "[CardsClient] flushPendingDeletes on beforeunload — may be cancelled by browser",
      );
      void flushPendingDeletes();
    }
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, []);

  async function handleUpdate(id: string, values: { front: string; back: string }) {
    const result = await updateCard({
      variables: { id, input: { front: values.front, back: values.back } },
    }).catch((err) => {
      // Structured log for operator triage: name + domain context only.
      // err.message is omitted — backend messages may carry user-authored content.
      // See .claude/rules/frontend-typescript-conventions.md § "expect.objectContaining".
      console.error("[CardsClient] update rejection", {
        name: err instanceof Error ? err.name : "unknown",
        cardgroupId,
        cardId: id,
      });
      return null;
    });
    if (result?.data?.updateCard?.card) {
      setEditingId(null);
    }
  }

  function handleClearSearch() {
    setSearchInput("");
    setSearchQuery(null);
  }

  const fetchingMore = networkStatus === NetworkStatus.fetchMore || (loading && edges.length > 0);
  const hasActiveSearch = searchQuery !== null;

  const addCardHref = `/cards/new?cardgroup=${encodeURIComponent(
    cardgroupId,
  )}&return=/cardgroups/${encodeURIComponent(cardgroupId)}/edit`;

  return (
    <div className="space-y-3">
      {queryBannerError && (
        <div
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="cards-query-error"
        >
          {queryBannerError}
        </div>
      )}

      {deleteCommitError && (
        <div
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="cards-delete-error"
        >
          {deleteCommitError}
        </div>
      )}

      {bulkDeleteBannerError && (
        <div
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="cards-bulk-delete-error"
        >
          {bulkDeleteBannerError}
        </div>
      )}

      <section>
        {sectionHeader === undefined ? (
          <h2 className="mb-3 text-sm font-medium uppercase tracking-wide text-muted-foreground">
            Cards ({totalCount})
          </h2>
        ) : typeof sectionHeader === "function" ? (
          sectionHeader({ totalCount })
        ) : (
          sectionHeader
        )}

        {/* Search input — debounced 300ms onto searchQuery */}
        <div className="mb-3">
          <div className="relative">
            <Search
              aria-hidden="true"
              className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            />
            <input
              type="search"
              placeholder="Search front or back..."
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              className="w-full rounded-md border border-input bg-background pl-8 pr-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              aria-label="Search cards"
              data-testid="cards-search-input"
            />
          </div>
        </div>

        {selectedIds.size > 0 && (
          <div
            className="mb-3 flex items-center gap-3 rounded-md border border-border bg-muted/50 px-4 py-2"
            data-testid="cards-bulk-action-bar"
          >
            <span className="flex-1 text-sm font-medium">{selectedIds.size} selected</span>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={bulkDeleting}
                  data-testid="cards-bulk-delete-button"
                >
                  Delete selected
                  <Trash2 aria-hidden="true" className="ml-1.5 h-4 w-4" />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete {selectedIds.size} cards?</AlertDialogTitle>
                  <AlertDialogDescription>This action cannot be undone.</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction data-testid="cards-bulk-confirm" onClick={handleBulkDelete}>
                    Delete {selectedIds.size}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
            <Button variant="outline" size="sm" onClick={clearSelection}>
              Cancel
              <X aria-hidden="true" className="ml-1.5 h-4 w-4" />
            </Button>
          </div>
        )}

        {edges.length === 0 ? (
          hasActiveSearch ? (
            <div
              className="flex flex-col items-center gap-3 rounded-md border border-dashed border-border p-6"
              data-testid="cards-empty-search"
            >
              <p className="text-sm text-muted-foreground">No cards match "{searchQuery}"</p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleClearSearch}
                data-testid="cards-clear-search"
              >
                Clear search
              </Button>
            </div>
          ) : (
            <div
              className="flex flex-col items-center gap-3 rounded-md border border-dashed border-border p-6"
              data-testid="cards-empty"
            >
              <p className="text-sm text-muted-foreground">Add some new cards to get started.</p>
              <Button asChild variant="brand" size="sm">
                <Link href={addCardHref} data-testid="cards-empty-add-card">
                  Add card
                  <Plus aria-hidden="true" className="ml-1.5 h-4 w-4" />
                </Link>
              </Button>
            </div>
          )
        ) : (
          <ul className="space-y-3">
            {edges.map((edge) => {
              const card = edge.node;
              return editingId === card.id ? (
                // biome-ignore lint/a11y/useKeyWithClickEvents: stopPropagation prevents bubbling to the parent's edit-toggle handler; this <li> is not interactive while CardForm is shown.
                <li
                  key={card.id}
                  className="rounded-md border border-border p-4"
                  // Prevent clicks on form fields from bubbling up to a parent
                  // edit toggle (defensive — the row-level edit trigger only
                  // exists in the non-editing branch).
                  onClick={(e) => e.stopPropagation()}
                >
                  <CardForm
                    mode="edit"
                    idPrefix={`edit-${card.id}-`}
                    defaultValues={{ front: card.front, back: card.back }}
                    submit={(values) => handleUpdate(card.id, values)}
                    submitLabel="Save"
                    submitting={updating}
                    error={updateError}
                    onCancel={() => setEditingId(null)}
                  />
                </li>
              ) : (
                <li key={card.id} className="rounded-md border border-border overflow-hidden">
                  <SwipeableRow
                    ref={(() => {
                      // Lazily create and cache a RefObject per card id so the
                      // ref identity is stable across re-renders. The Map lives
                      // in rowRefs.current and is never cleared during the list's
                      // lifetime (entries for removed cards become garbage-collected
                      // when the card is no longer in `edges`).
                      if (!rowRefs.current.has(card.id)) {
                        rowRefs.current.set(card.id, { current: null });
                      }
                      // biome-ignore lint/style/noNonNullAssertion: we just set the entry above so it is always defined.
                      return rowRefs.current.get(card.id)!;
                    })()}
                    onDelete={() => handleDeleteRow(card.id)}
                    // Disable swipe while selection mode is active so checkboxes
                    // receive touch events without interference.
                    disabled={selectedIds.size > 0 || editingId === card.id}
                    ariaLabel="Delete card"
                  >
                    <div className="group flex items-start justify-between gap-4 px-4 py-3">
                      {/* Checkbox cell — clicks must NOT bubble to the row edit trigger.
                          The handlers are no-op stoppers, not user-facing interactivity;
                          the inner <input> remains the keyboard/mouse target. */}
                      {/* biome-ignore lint/a11y/noStaticElementInteractions: span is a click/keydown stopper, not an interactive element; the inner <input> is the actual control. */}
                      <span
                        onClick={(e) => e.stopPropagation()}
                        onKeyDown={(e) => e.stopPropagation()}
                        className="mt-0.5 shrink-0"
                      >
                        <input
                          type="checkbox"
                          className="h-4 w-4 cursor-pointer accent-primary"
                          checked={selectedIds.has(card.id)}
                          onChange={() => toggleSelected(card.id)}
                          aria-label="Select card"
                          data-testid={`card-select-${card.id}`}
                        />
                      </span>
                      {/* Text region acts as the edit trigger. role/tabIndex/onKeyDown
                          let keyboard users (Tab + Enter/Space) enter edit mode.
                          Close any half-open sibling row before entering edit mode. */}
                      {/* biome-ignore lint/a11y/useSemanticElements: a native <button> here would nest the action <button> for delete (invalid HTML); role="button" preserves screen-reader semantics without the markup conflict. */}
                      <div
                        role="button"
                        tabIndex={0}
                        onClick={() => {
                          closeOtherRows(card.id);
                          setEditingId(card.id);
                        }}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            closeOtherRows(card.id);
                            setEditingId(card.id);
                          }
                        }}
                        className="min-w-0 flex-1 cursor-pointer space-y-1 rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        aria-label={`Edit card ${card.front}`}
                        data-testid={`card-edit-target-${card.id}`}
                      >
                        <p className="text-sm font-medium">{card.front}</p>
                        <p className="text-sm text-muted-foreground">{card.back}</p>
                      </div>
                      {/* biome-ignore lint/a11y/noStaticElementInteractions: div is a click/keydown stopper, not an interactive element; the inner Delete <button> is the actual control. */}
                      <div
                        className="flex shrink-0 gap-2"
                        onClick={(e) => e.stopPropagation()}
                        onKeyDown={(e) => e.stopPropagation()}
                      >
                        <Button
                          variant="outline"
                          size="icon"
                          aria-label="Delete card"
                          onClick={() => handleDeleteRow(card.id)}
                          data-testid={`card-delete-${card.id}`}
                          className="opacity-100 sm:opacity-0 sm:transition-opacity sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"
                        >
                          <Trash2 aria-hidden="true" className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  </SwipeableRow>
                </li>
              );
            })}
          </ul>
        )}

        <div ref={sentinelRef} aria-hidden="true" data-testid="cards-sentinel" />
        {fetchMoreError && (
          <div
            className="mt-3 flex flex-col items-center gap-2 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
            role="alert"
            data-testid="cards-fetch-more-error"
          >
            <span>{fetchMoreError}</span>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => {
                setFetchMoreError(null);
                requestNextPage();
              }}
            >
              Retry
            </Button>
          </div>
        )}
        {!fetchMoreError && fetchingMore && pageInfo.hasNextPage && (
          <p className="mt-3 text-center text-xs text-muted-foreground">Loading more cards...</p>
        )}
      </section>
    </div>
  );
}
