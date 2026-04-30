"use client";

import { gql, NetworkStatus } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  CreateCardMutation,
  DeleteCardMutation,
  DeleteCardsMutation,
  UpdateCardMutation,
} from "@/app/cardgroups/queries";
import { CardForm } from "@/components/cardgroups/card-form";
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
} from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { CARDS_PAGE_SIZE } from "./queries";

type Connection = CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"];
type Edge = Connection["edges"][number];
type PageInfo = Connection["pageInfo"];

type Props = {
  cardgroupId: string;
  initialEdges: Edge[];
  initialPageInfo: PageInfo;
  initialTotalCount: number;
};

export function CardsClient({
  cardgroupId,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
}: Props) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [createFormKey, setCreateFormKey] = useState(0);
  const [fetchMoreError, setFetchMoreError] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

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

  const {
    data,
    fetchMore,
    loading,
    networkStatus,
    error: queryError,
  } = useQuery(CardsByCardgroupConnectionDocument, {
    variables: { cardgroupId, first: CARDS_PAGE_SIZE },
    fetchPolicy: "cache-first",
    notifyOnNetworkStatusChange: true,
  });

  const queryBannerError = getBackendErrorBanner(queryError);

  const connection = data?.cardsByCardgroupConnection;
  const edges = connection?.edges ?? initialEdges;
  const pageInfo = connection?.pageInfo ?? initialPageInfo;
  const totalCount = connection?.totalCount ?? initialTotalCount;

  const sentinelRef = useRef<HTMLDivElement | null>(null);
  const fetchingRef = useRef(false);

  const requestNextPage = useCallback(() => {
    if (fetchingRef.current) return;
    if (!pageInfo.hasNextPage) return;

    fetchingRef.current = true;
    fetchMore({
      variables: {
        cardgroupId,
        first: CARDS_PAGE_SIZE,
        after: pageInfo.endCursor,
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
        const banner = getBackendErrorBanner(err) ?? "Could not load more cards. Please try again.";
        setFetchMoreError(banner);
      })
      .finally(() => {
        fetchingRef.current = false;
      });
  }, [cardgroupId, fetchMore, pageInfo.endCursor, pageInfo.hasNextPage]);

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
      if (!pageInfo.hasNextPage) return;
      requestNextPage();
    });

    observer.observe(node);
    return () => observer.disconnect();
  }, [pageInfo.hasNextPage, fetchMoreError, requestNextPage]);

  const [createCard, { loading: creating, error: createError }] = useMutation(CreateCardMutation, {
    update(cache, { data: createData }) {
      if (!createData?.createCard?.card) return;
      const newCard = createData.createCard.card;
      // Write the new Card into the cache so any edge that references it resolves correctly.
      cache.writeFragment({
        id: cache.identify({ __typename: "Card", id: newCard.id }),
        fragment: gql`
          fragment NewCardFields on Card {
            id
            front
            back
            due
            state
            cardgroupId
          }
        `,
        data: newCard,
      });
      // Use readQuery/writeQuery so cold caches (no existing connection entry) get
      // a freshly-written connection — cache.modify silently no-ops when the field
      // is missing, which would lose the new card on first load.
      const variables = { cardgroupId, first: CARDS_PAGE_SIZE };
      const existing = cache.readQuery({
        query: CardsByCardgroupConnectionDocument,
        variables,
      });
      const newEdge = {
        __typename: "CardEdge" as const,
        cursor: newCard.id,
        node: newCard,
      };
      const next = existing
        ? {
            cardsByCardgroupConnection: {
              ...existing.cardsByCardgroupConnection,
              edges: [...existing.cardsByCardgroupConnection.edges, newEdge],
              totalCount: existing.cardsByCardgroupConnection.totalCount + 1,
            },
          }
        : {
            cardsByCardgroupConnection: {
              __typename: "CardConnection" as const,
              edges: [newEdge],
              pageInfo: {
                __typename: "PageInfo" as const,
                hasNextPage: false,
                hasPreviousPage: false,
                startCursor: newCard.id,
                endCursor: newCard.id,
              },
              totalCount: 1,
            },
          };
      cache.writeQuery({
        query: CardsByCardgroupConnectionDocument,
        variables,
        data: next,
      });
      setCreateFormKey((k) => k + 1);
    },
  });

  // Update propagates automatically via Apollo cache normalization (Card has id).
  const [updateCard, { loading: updating, error: updateError }] = useMutation(UpdateCardMutation);

  const [deleteCard, { error: deleteError }] = useMutation(DeleteCardMutation, {
    update(cache, { data: deleteData }, { variables: deleteVars }) {
      if (!deleteData?.deleteCard) return;
      const id = deleteVars?.id as string | undefined;
      if (!id) return;
      // Use readQuery/writeQuery for cold-cache safety: when no connection has been
      // cached for this cardgroup yet, there is nothing to remove and we leave the
      // cache untouched. When the connection exists, drop any matching edge and
      // decrement totalCount unconditionally (the deleted card may live on a page
      // that was never fetched into edges).
      const variables = { cardgroupId, first: CARDS_PAGE_SIZE };
      const existing = cache.readQuery({
        query: CardsByCardgroupConnectionDocument,
        variables,
      });
      if (existing) {
        const filteredEdges = existing.cardsByCardgroupConnection.edges.filter(
          (edge) => edge.node.id !== id,
        );
        cache.writeQuery({
          query: CardsByCardgroupConnectionDocument,
          variables,
          data: {
            cardsByCardgroupConnection: {
              ...existing.cardsByCardgroupConnection,
              edges: filteredEdges,
              totalCount: Math.max(0, existing.cardsByCardgroupConnection.totalCount - 1),
            },
          },
        });
      }
      cache.evict({ id: cache.identify({ __typename: "Card", id }) });
      cache.gc();
    },
  });

  const deleteBannerError = getBackendErrorBanner(deleteError);

  const [deleteCards, { error: bulkDeleteError, loading: bulkDeleting }] = useMutation(
    DeleteCardsMutation,
    {
      update(cache, { data }, { variables }) {
        const ids = variables?.ids as string[] | undefined;
        if (!ids) return;
        // Guard: only proceed when the backend returned a real response.
        // If data is undefined (e.g. network failure) or deleteCards is null,
        // return early — do NOT evict, do NOT writeQuery, do NOT gc.
        // This mirrors the single-delete callback which returns early on
        // !deleteData?.deleteCard.
        if (data?.deleteCards == null) return;
        const deletedCount = data.deleteCards;

        // Backend reports actual rows deleted; some ids may have been skipped (foreign-owned).
        const variables2 = { cardgroupId, first: CARDS_PAGE_SIZE };
        const existing = cache.readQuery({
          query: CardsByCardgroupConnectionDocument,
          variables: variables2,
        });

        if (existing && deletedCount > 0) {
          const filteredEdges = existing.cardsByCardgroupConnection.edges.filter(
            (edge) => !ids.includes(edge.node.id),
          );
          cache.writeQuery({
            query: CardsByCardgroupConnectionDocument,
            variables: variables2,
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

        // Only evict normalized entries when the backend confirmed at least one
        // row was actually deleted — there is no point evicting when deletedCount
        // is 0 (the user's selection may contain ids that never existed locally).
        if (deletedCount > 0) {
          for (const id of ids) {
            cache.evict({ id: cache.identify({ __typename: "Card", id }) });
          }
          cache.gc();
        }
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
      console.error("[CardsClient] bulk delete rejection", err);
    }
  }

  async function handleCreate(values: { front: string; back: string }) {
    await createCard({
      variables: { input: { cardgroupId, front: values.front, back: values.back } },
    }).catch((err) => {
      console.error("[CardsClient] create rejection", err);
    });
  }

  async function handleUpdate(id: string, values: { front: string; back: string }) {
    const result = await updateCard({
      variables: { id, input: { front: values.front, back: values.back } },
    }).catch((err) => {
      console.error("[CardsClient] update rejection", err);
      return null;
    });
    if (result?.data?.updateCard?.card) {
      setEditingId(null);
    }
  }

  const fetchingMore = networkStatus === NetworkStatus.fetchMore || (loading && edges.length > 0);

  return (
    <div className="space-y-6">
      {queryBannerError && (
        <div
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
          data-testid="cards-query-error"
        >
          {queryBannerError}
        </div>
      )}

      {deleteBannerError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {deleteBannerError}
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
        <h2 className="mb-3 text-sm font-medium uppercase tracking-wide text-muted-foreground">
          Add a card
        </h2>
        <CardForm
          key={createFormKey}
          mode="create"
          idPrefix="add-"
          defaultValues={{ front: "", back: "" }}
          submit={handleCreate}
          submitLabel="Add"
          submitting={creating}
          error={createError}
        />
      </section>

      <section>
        <h2 className="mb-3 text-sm font-medium uppercase tracking-wide text-muted-foreground">
          Cards ({totalCount})
        </h2>

        {selectedIds.size > 0 && (
          <div
            className="mb-3 flex items-center gap-3 rounded-md border border-border bg-muted/50 px-4 py-2"
            data-testid="cards-bulk-action-bar"
          >
            <span className="flex-1 text-sm font-medium">{selectedIds.size} selected</span>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button
                  variant="destructive"
                  size="sm"
                  disabled={bulkDeleting}
                  data-testid="cards-bulk-delete-button"
                >
                  Delete selected
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
            </Button>
          </div>
        )}

        {edges.length === 0 ? (
          <p className="text-sm text-muted-foreground">No cards yet. Add one above.</p>
        ) : (
          <ul className="space-y-3">
            {edges.map((edge) => {
              const card = edge.node;
              return editingId === card.id ? (
                <li key={card.id} className="rounded-md border border-border p-4">
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
                <li
                  key={card.id}
                  className="flex items-start justify-between gap-4 rounded-md border border-border px-4 py-3"
                >
                  <input
                    type="checkbox"
                    className="mt-0.5 h-4 w-4 shrink-0 cursor-pointer accent-primary"
                    checked={selectedIds.has(card.id)}
                    onChange={() => toggleSelected(card.id)}
                    aria-label="Select card"
                    data-testid={`card-select-${card.id}`}
                  />
                  <div className="min-w-0 flex-1 space-y-1">
                    <p className="text-sm font-medium">{card.front}</p>
                    <p className="text-sm text-muted-foreground">{card.back}</p>
                  </div>
                  <div className="flex shrink-0 gap-2">
                    <Button variant="outline" size="sm" onClick={() => setEditingId(card.id)}>
                      Edit
                    </Button>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="destructive" size="sm">
                          Delete
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Delete card?</AlertDialogTitle>
                          <AlertDialogDescription>
                            This action cannot be undone.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction
                            onClick={async () => {
                              await deleteCard({ variables: { id: card.id } }).catch(console.error);
                            }}
                          >
                            Delete
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
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
