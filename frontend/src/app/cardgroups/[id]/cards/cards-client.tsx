"use client";

import { useApolloClient, useMutation } from "@apollo/client/react";
import { Search } from "lucide-react";
import { useTranslations } from "next-intl";
import type { ReactNode, RefObject } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  CreateCardMutation,
  DeleteCardMutation,
  DeleteCardsMutation,
  UpdateCardMutation,
} from "@/app/cardgroups/queries";
import { CardForm } from "@/components/cardgroups/card-form";
import { CardgroupBatchImportForm } from "@/components/cardgroups/cardgroup-batch-import-form";
import type { SwipeableRowHandle } from "@/components/cardgroups/swipeable-row";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import { useBulkSelection } from "@/hooks/use-bulk-selection";
import { useDebouncedSearch } from "@/hooks/use-debounced-search";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { useUndoDelete } from "@/lib/undo-delete";
import { BulkActionBar } from "./components/bulk-action-bar";
import { CardRow } from "./components/card-row";
import { cardsDefaultVars } from "./queries";
import { useCardsConnection } from "./use-cards-connection";

type Connection = CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"];
export type CardEdge = Connection["edges"][number];
export type CardConnectionPageInfo = Connection["pageInfo"];

const FetchMoreError = ({ message, onRetry }: { message: string; onRetry: () => void }) => {
  const tCommon = useTranslations("Common");
  return (
    <ErrorBanner
      className="mt-3 flex flex-col items-center gap-2"
      data-testid="cards-fetch-more-error"
    >
      <span>{message}</span>
      <Button type="button" variant="outline" size="sm" onClick={onRetry}>
        {tCommon("retry")}
      </Button>
    </ErrorBanner>
  );
};

const SearchInput = ({ value, onChange }: { value: string; onChange: (next: string) => void }) => {
  const t = useTranslations("Cards");
  return (
    <div className="mb-3">
      <div className="relative">
        <Search
          aria-hidden="true"
          className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
        />
        <input
          type="search"
          placeholder={t("searchPlaceholder")}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-full rounded-md border border-input bg-background pl-8 pr-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={t("searchAriaLabel")}
          data-testid="cards-search-input"
        />
      </div>
    </div>
  );
};

function EditCardSheetContent({
  card,
  submit,
  submitting,
  error,
  validationError,
}: {
  card: { id: string; front: string; back: string };
  submit: (values: { front: string; back: string }) => Promise<void>;
  submitting: boolean;
  error: unknown;
  validationError: { field: string; message: string } | null;
}) {
  const close = useFormSheetClose();
  const tCommon = useTranslations("Common");

  return (
    <CardForm
      mode="edit"
      idPrefix={`edit-${card.id}-`}
      defaultValues={{ front: card.front, back: card.back }}
      submit={submit}
      submitLabel={tCommon("save")}
      submitting={submitting}
      error={error}
      validationError={validationError}
      onCancel={close}
    />
  );
}

function AddCardSheetContent({
  submit,
  submitting,
  error,
  validationError,
  onDirty,
}: {
  submit: (values: { front: string; back: string }) => Promise<void>;
  submitting: boolean;
  error: unknown;
  validationError: { field: string; message: string } | null;
  onDirty: () => void;
}) {
  const close = useFormSheetClose();

  return (
    <div onInput={onDirty}>
      <CardForm
        mode="create"
        idPrefix="add-card-"
        defaultValues={{ front: "", back: "" }}
        submit={submit}
        submitting={submitting}
        error={error}
        validationError={validationError}
        onCancel={close}
      />
    </div>
  );
}

const EmptyState = ({ search, onClear }: { search: string | null; onClear: () => void }) => {
  const t = useTranslations("Cards");
  return search !== null ? (
    <div
      className="flex flex-col items-center gap-3 rounded-md border border-dashed border-border p-6"
      data-testid="cards-empty-search"
    >
      <p className="text-sm text-muted-foreground">{t("noMatch", { search })}</p>
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={onClear}
        data-testid="cards-clear-search"
      >
        {t("clearSearch")}
      </Button>
    </div>
  ) : (
    <div
      className="flex flex-col items-center gap-3 rounded-md border border-dashed border-border p-6"
      data-testid="cards-empty"
    >
      <p className="text-sm text-muted-foreground">{t("addSomeCards")}</p>
    </div>
  );
};

type SectionHeaderArgs = {
  totalCount: number;
  onAddCard: () => void;
  onBatchImport: () => void;
};

type Props = {
  cardgroupId: string;
  cardgroupName: string;
  initialEdges: CardEdge[];
  initialPageInfo: CardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Optional section header. When `undefined`, the default `<h2>Cards (n)</h2>`
   * is rendered. Pass a `ReactNode` to replace the header, `null` to suppress
   * it entirely, or a render function to access the live `totalCount` from
   * Apollo cache without spinning up a second `useQuery` in the parent.
   */
  sectionHeader?: ReactNode | ((args: SectionHeaderArgs) => ReactNode);
};

export function CardsClient({
  cardgroupId,
  cardgroupName,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
  sectionHeader,
}: Props) {
  const t = useTranslations("Cards");
  const apollo = useApolloClient();
  const { scheduleDelete } = useUndoDelete();
  const [addOpen, setAddOpen] = useState(false);
  const [addDirty, setAddDirty] = useState(false);
  const [batchImportOpen, setBatchImportOpen] = useState(false);
  const [createValidationError, setCreateValidationError] = useState<{
    field: string;
    message: string;
  } | null>(null);
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

  const editingCard = edges.find((edge) => edge.node.id === editingId)?.node;

  const [createCard, { loading: creating, error: createError, reset: resetCreateCard }] =
    useMutation(CreateCardMutation);
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

  const openAddSheet = useCallback(() => {
    resetCreateCard();
    setCreateValidationError(null);
    setAddDirty(false);
    setAddOpen(true);
  }, [resetCreateCard]);

  const openBatchImport = useCallback(() => setBatchImportOpen(true), []);
  const closeBatchImport = useCallback(() => setBatchImportOpen(false), []);

  useEffect(() => {
    function handleAddCardEvent(event: Event) {
      if (!(event instanceof CustomEvent)) return;
      const detail = event.detail as { cardgroupId?: unknown } | null;
      if (detail?.cardgroupId !== cardgroupId) return;

      event.preventDefault();
      openAddSheet();
    }

    window.addEventListener("flamingo:add-card", handleAddCardEvent);
    return () => window.removeEventListener("flamingo:add-card", handleAddCardEvent);
  }, [cardgroupId, openAddSheet]);

  function cardMatchesSearch(
    card: CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"]["edges"][number]["node"],
    searchValue: string,
  ) {
    const normalized = searchValue.trim().toLowerCase();
    if (normalized === "") return true;
    return (
      card.front.toLowerCase().includes(normalized) || card.back.toLowerCase().includes(normalized)
    );
  }

  function writeCreatedCardToConnection(
    card: CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"]["edges"][number]["node"],
    variables: CardsByCardgroupConnectionQueryVariables,
  ) {
    const existing = apollo.readQuery({
      query: CardsByCardgroupConnectionDocument,
      variables,
    });
    if (!existing) return;

    const next = existing.cardsByCardgroupConnection;
    if (next.edges.some((edge) => edge.node.id === card.id)) return;

    apollo.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables,
      data: {
        cardsByCardgroupConnection: {
          ...next,
          edges: [{ __typename: "CardEdge" as const, cursor: card.id, node: card }, ...next.edges],
          totalCount: next.totalCount + 1,
        },
      },
    });
  }

  async function handleCreate(values: { front: string; back: string }) {
    resetCreateCard();
    setCreateValidationError(null);
    const result = await createCard({
      variables: { input: { cardgroupId, front: values.front, back: values.back } },
    }).catch((err) => {
      console.error("[CardsClient] create rejection", {
        name: err instanceof Error ? err.name : "unknown",
        cardgroupId,
      });
      return null;
    });
    if (!result) return;

    const payload = result.data?.createCard;
    if (payload?.__typename === "CreateCardSuccess") {
      writeCreatedCardToConnection(payload.card, cardsDefaultVars(cardgroupId));
      if (
        typeof queryVariables.search === "string" &&
        cardMatchesSearch(payload.card, queryVariables.search)
      ) {
        writeCreatedCardToConnection(payload.card, queryVariables);
      }
      setAddDirty(false);
      setCreateValidationError(null);
      setAddOpen(false);
    } else if (payload?.__typename === "CardDuplicateFrontError") {
      setCreateValidationError({ field: "front", message: payload.message });
    } else {
      const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
      console.warn("[CardsClient] unexpected createCard payload", {
        typename: unknownPayload?.__typename ?? null,
        cardgroupId,
      });
      setCreateValidationError({ field: "front", message: t("addFailed") });
    }
  }

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
        label: t("cardDeleted"),
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
          setDeleteCommitError(getBackendErrorBanner(err) ?? t("deleteError"));
        },
      });
    },
    [apollo, deleteCardMutation, queryVariables, scheduleDelete, t],
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
      setRowValidationError({ field: "front", message: t("saveFailed") });
    }
  }

  return (
    <div className="space-y-3">
      {queryBannerError && (
        <ErrorBanner data-testid="cards-query-error">{queryBannerError}</ErrorBanner>
      )}
      {deleteCommitError && (
        <ErrorBanner data-testid="cards-delete-error">{deleteCommitError}</ErrorBanner>
      )}
      {bulkDeleteBannerError && (
        <ErrorBanner data-testid="cards-bulk-delete-error">{bulkDeleteBannerError}</ErrorBanner>
      )}

      <section>
        {sectionHeader === undefined ? (
          <h2 className="mb-3 text-sm font-medium uppercase tracking-wide text-muted-foreground">
            {t("cardsCount", { count: totalCount })}
          </h2>
        ) : typeof sectionHeader === "function" ? (
          sectionHeader({ totalCount, onAddCard: openAddSheet, onBatchImport: openBatchImport })
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
              return (
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

        <FormSheet
          title={t("addCard")}
          open={addOpen}
          onOpenChange={(nextOpen) => {
            setAddOpen(nextOpen);
            if (!nextOpen) {
              resetCreateCard();
              setAddDirty(false);
              setCreateValidationError(null);
            }
          }}
          submitting={creating}
          dirty={addDirty}
          confirmOnDismiss
        >
          <AddCardSheetContent
            submit={handleCreate}
            submitting={creating}
            error={createError}
            validationError={createValidationError}
            onDirty={() => setAddDirty(true)}
          />
        </FormSheet>

        {/* Object-first: header shows only the destination cardgroup (the high-risk variable); the verb lives in the sr-only accessible name. Intentional deviation from the verb-first sheets. */}
        <FormSheet
          title={
            <span
              className="block overflow-hidden text-ellipsis whitespace-nowrap"
              title={cardgroupName}
            >
              <span className="sr-only">{t("batchImportSrOnly")}</span>
              {cardgroupName}
            </span>
          }
          open={batchImportOpen}
          onOpenChange={setBatchImportOpen}
          size="lg"
        >
          <CardgroupBatchImportForm
            cardgroupId={cardgroupId}
            cardgroupName={cardgroupName}
            onImported={closeBatchImport}
            onCancel={closeBatchImport}
          />
        </FormSheet>

        <FormSheet
          title={t("editCard")}
          open={editingCard !== undefined}
          onOpenChange={(nextOpen) => {
            if (!nextOpen) {
              setRowValidationError(null);
              setEditingId(null);
            }
          }}
          submitting={updating}
          confirmOnDismiss={false}
        >
          {editingCard ? (
            <EditCardSheetContent
              card={editingCard}
              submit={(values) => handleUpdate(editingCard.id, values)}
              submitting={updating}
              error={updateError}
              validationError={rowValidationError}
            />
          ) : null}
        </FormSheet>

        <div ref={sentinelRef} aria-hidden="true" data-testid="cards-sentinel" />
        {fetchMoreError && <FetchMoreError message={fetchMoreError} onRetry={retryFetchMore} />}
        {!fetchMoreError && fetchingMore && pageInfo.hasNextPage && (
          <p className="mt-3 text-center text-xs text-muted-foreground">{t("loadingMore")}</p>
        )}
      </section>
    </div>
  );
}
