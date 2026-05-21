"use client";

import { useApolloClient, useMutation } from "@apollo/client/react";
import { Search } from "lucide-react";
import type { ReactNode, RefObject } from "react";
import { useCallback, useRef, useState } from "react";
import {
  DeleteCardMutation,
  DeleteCardsMutation,
  UpdateCardMutation,
} from "@/app/cardgroups/queries";
import type { SwipeableRowHandle } from "@/components/cardgroups/swipeable-row";
import { Button } from "@/components/ui/button";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
} from "@/generated/graphql";
import { useBulkSelection } from "@/hooks/use-bulk-selection";
import { useDebouncedSearch } from "@/hooks/use-debounced-search";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { useUndoDelete } from "@/lib/undo-delete";
import { BulkActionBar } from "./components/bulk-action-bar";
import { CardRow } from "./components/card-row";
import { EditCardRow } from "./components/edit-card-row";
import { useCardsConnection } from "./use-cards-connection";

type Connection = CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"];
export type CardEdge = Connection["edges"][number];
export type CardConnectionPageInfo = Connection["pageInfo"];

const ErrorBanner = ({ testId, message }: { testId: string; message: string }) => (
  <div
    className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
    role="alert"
    data-testid={testId}
  >
    {message}
  </div>
);

const FetchMoreError = ({ message, onRetry }: { message: string; onRetry: () => void }) => (
  <div
    className="mt-3 flex flex-col items-center gap-2 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
    role="alert"
    data-testid="cards-fetch-more-error"
  >
    <span>{message}</span>
    <Button type="button" variant="outline" size="sm" onClick={onRetry}>
      Retry
    </Button>
  </div>
);

const SearchInput = ({ value, onChange }: { value: string; onChange: (next: string) => void }) => (
  <div className="mb-3">
    <div className="relative">
      <Search
        aria-hidden="true"
        className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
      />
      <input
        type="search"
        placeholder="Search front or back..."
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-md border border-input bg-background pl-8 pr-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        aria-label="Search cards"
        data-testid="cards-search-input"
      />
    </div>
  </div>
);

const EmptyState = ({ search, onClear }: { search: string | null; onClear: () => void }) =>
  search !== null ? (
    <div
      className="flex flex-col items-center gap-3 rounded-md border border-dashed border-border p-6"
      data-testid="cards-empty-search"
    >
      <p className="text-sm text-muted-foreground">No cards match "{search}"</p>
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={onClear}
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
    </div>
  );

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
  const { scheduleDelete } = useUndoDelete();
  const [editingId, setEditingId] = useState<string | null>(null);
  // Field-level error for the editing row (from updateCard's outcome-union).
  const [rowValidationError, setRowValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);
  // Per-card SwipeableRow handles; lets a row close any half-open sibling.
  const rowRefs = useRef<Map<string, RefObject<SwipeableRowHandle | null>>>(new Map());
  const selection = useBulkSelection<string>();
  const search = useDebouncedSearch();
  // Per-row delete commit error banner persists across useMutation calls.
  // See docs/pagination/do-not-reuse-mutation-error-state.md.
  const [deleteCommitError, setDeleteCommitError] = useState<string | null>(null);

  const {
    edges,
    pageInfo,
    totalCount,
    loading,
    fetchingMore,
    fetchMoreError,
    retryFetchMore,
    sentinelRef,
    queryVariables,
    queryError,
  } = useCardsConnection({
    cardgroupId,
    searchQuery: search.query,
    initialEdges,
    initialPageInfo,
    initialTotalCount,
  });

  const queryBannerError = getBackendErrorBanner(queryError);

  const closeOtherRows = useCallback((exceptCardId: string) => {
    for (const [id, ref] of rowRefs.current.entries()) {
      if (id !== exceptCardId) ref.current?.close();
    }
  }, []);

  // Updates propagate automatically via Apollo cache normalization (Card has id).
  const [updateCard, { loading: updating, error: updateError }] = useMutation(UpdateCardMutation);
  // scheduleDelete (5s undo window) fires the per-row mutation imperatively.
  const [deleteCardMutation] = useMutation(DeleteCardMutation);

  const [deleteCards, { error: bulkDeleteError, loading: bulkDeleting }] = useMutation(
    DeleteCardsMutation,
    {
      // queryVariables matches the live cache entry under any active search filter.
      // See docs/pagination/optimistic-rollback-cache-key.md.
      update(cache, { data: bulkData }, { variables: mutationVars }) {
        const ids = mutationVars?.ids as string[] | undefined;
        if (!ids) return;
        if (bulkData?.deleteCards == null) return;
        const deletedCount = bulkData.deleteCards;
        if (deletedCount === 0) return;
        const existing = cache.readQuery({
          query: CardsByCardgroupConnectionDocument,
          variables: queryVariables,
        });
        if (existing) {
          const next = existing.cardsByCardgroupConnection;
          cache.writeQuery({
            query: CardsByCardgroupConnectionDocument,
            variables: queryVariables,
            data: {
              cardsByCardgroupConnection: {
                ...next,
                edges: next.edges.filter((edge) => !ids.includes(edge.node.id)),
                totalCount: Math.max(0, next.totalCount - deletedCount),
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
    const ids = Array.from(selection.selectedIds);
    try {
      await deleteCards({ variables: { ids } });
      selection.clearSelection();
    } catch (err) {
      console.error("[CardsClient] bulk delete rejection", {
        name: err instanceof Error ? err.name : "unknown",
        cardgroupId,
        ids,
      });
    }
  }

  // Per-row delete: snapshot, optimistic drop, schedule DELETE with a 5s undo window.
  // See docs/pagination/optimistic-rollback-cache-key.md for the queryVariables rule.
  const handleDeleteRow = useCallback(
    (cardId: string) => {
      const snapshot = apollo.readQuery({
        query: CardsByCardgroupConnectionDocument,
        variables: queryVariables,
      });
      if (snapshot) {
        const next = snapshot.cardsByCardgroupConnection;
        apollo.writeQuery({
          query: CardsByCardgroupConnectionDocument,
          variables: queryVariables,
          data: {
            cardsByCardgroupConnection: {
              ...next,
              edges: next.edges.filter((edge) => edge.node.id !== cardId),
              totalCount: Math.max(0, next.totalCount - 1),
            },
          },
        });
      }
      scheduleDelete({
        id: cardId,
        label: "Card deleted",
        optimisticRollback: () => {
          if (snapshot !== null) {
            apollo.writeQuery({
              query: CardsByCardgroupConnectionDocument,
              variables: queryVariables,
              data: snapshot,
            });
          }
          setDeleteCommitError(null);
        },
        commitDelete: async () => {
          setDeleteCommitError(null);
          const result = await deleteCardMutation({ variables: { id: cardId } });
          if (result.data?.deleteCard) {
            apollo.cache.evict({ id: apollo.cache.identify({ __typename: "Card", id: cardId }) });
            apollo.cache.gc();
          }
        },
        onCommitFailed: (err) => {
          setDeleteCommitError(
            getBackendErrorBanner(err) ?? "Could not delete card. Please try again.",
          );
        },
      });
    },
    [apollo, deleteCardMutation, queryVariables, scheduleDelete],
  );

  async function handleUpdate(id: string, values: { front: string; back: string }) {
    setRowValidationError(null);
    const result = await updateCard({
      variables: { id, input: { front: values.front, back: values.back } },
    }).catch((err) => {
      console.error("[CardsClient] update rejection", {
        name: err instanceof Error ? err.name : "unknown",
        cardgroupId,
        cardId: id,
      });
      return null;
    });
    if (!result) return;
    const payload = result.data?.updateCard;
    if (payload?.__typename === "UpdateCardSuccess") {
      setEditingId(null);
    } else if (payload?.__typename === "InputValidationError") {
      setRowValidationError({ field: payload.field, message: payload.message });
    } else {
      // Null payload or a future union member from a stale codegen.
      const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
      console.warn("[CardsClient] unexpected updateCard payload", {
        typename: unknownPayload?.__typename ?? null,
        cardId: id,
        cardgroupId,
      });
      setRowValidationError({ field: "front", message: "Save failed. Please try again." });
    }
  }

  return (
    <div className="space-y-3">
      {queryBannerError && <ErrorBanner testId="cards-query-error" message={queryBannerError} />}
      {deleteCommitError && <ErrorBanner testId="cards-delete-error" message={deleteCommitError} />}
      {bulkDeleteBannerError && (
        <ErrorBanner testId="cards-bulk-delete-error" message={bulkDeleteBannerError} />
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

        <SearchInput value={search.input} onChange={search.setInput} />

        {selection.count > 0 && (
          <BulkActionBar
            count={selection.count}
            busy={bulkDeleting}
            onConfirm={handleBulkDelete}
            onClear={selection.clearSelection}
          />
        )}

        {edges.length === 0 ? (
          <EmptyState search={search.query} onClear={search.clear} />
        ) : (
          <ul className="space-y-3">
            {edges.map((edge) => {
              const card = edge.node;
              return editingId === card.id ? (
                // biome-ignore lint/a11y/useKeyWithClickEvents: stopPropagation prevents bubbling to the parent's edit-toggle handler; this <li> is not interactive while CardForm is shown.
                <li
                  key={card.id}
                  className="rounded-md border border-border p-4"
                  onClick={(e) => e.stopPropagation()}
                >
                  <EditCardRow
                    card={card}
                    submit={(values) => handleUpdate(card.id, values)}
                    submitting={updating}
                    error={updateError}
                    validationError={rowValidationError}
                    onCancel={() => {
                      setRowValidationError(null);
                      setEditingId(null);
                    }}
                  />
                </li>
              ) : (
                <li key={card.id} className="rounded-md border border-border overflow-hidden">
                  <CardRow
                    card={card}
                    rowRef={(() => {
                      if (!rowRefs.current.has(card.id)) {
                        rowRefs.current.set(card.id, { current: null });
                      }
                      // biome-ignore lint/style/noNonNullAssertion: we just set the entry above so it is always defined.
                      return rowRefs.current.get(card.id)!;
                    })()}
                    selected={selection.isSelected(card.id)}
                    disabled={selection.count > 0 || editingId === card.id}
                    onSelectToggle={() => selection.toggleSelected(card.id)}
                    onEdit={() => {
                      closeOtherRows(card.id);
                      setRowValidationError(null);
                      setEditingId(card.id);
                    }}
                    onDelete={() => handleDeleteRow(card.id)}
                  />
                </li>
              );
            })}
          </ul>
        )}

        <div ref={sentinelRef} aria-hidden="true" data-testid="cards-sentinel" />
        {fetchMoreError && <FetchMoreError message={fetchMoreError} onRetry={retryFetchMore} />}
        {!fetchMoreError && fetchingMore && pageInfo.hasNextPage && (
          <p className="mt-3 text-center text-xs text-muted-foreground">Loading more cards...</p>
        )}
      </section>
    </div>
  );
}
