"use client";

import { Search } from "lucide-react";
import { useTranslations } from "next-intl";
import type { RefObject } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { BulkActionBar } from "@/components/cardgroups/bulk-action-bar";
import { CardForm } from "@/components/cardgroups/card-form";
import { CardRow } from "@/components/cardgroups/card-row";
import type { SwipeableRowHandle } from "@/components/cardgroups/swipeable-row";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import type { AdminMasterCardsConnectionQuery } from "@/generated/graphql";
import { useBulkSelection } from "@/hooks/use-bulk-selection";
import { useDebouncedSearch } from "@/hooks/use-debounced-search";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { MasterBatchImportForm } from "./master-batch-import-form";
import { useMasterCardMutations } from "./use-master-card-mutations";
import { useMasterCardsConnection } from "./use-master-cards-connection";

type Connection = AdminMasterCardsConnectionQuery["adminMasterCardsConnection"];
export type MasterCardEdge = Connection["edges"][number];
export type MasterCardConnectionPageInfo = Connection["pageInfo"];

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

type Props = {
  masterId: string;
  deckName: string;
  initialEdges: MasterCardEdge[];
  initialPageInfo: MasterCardConnectionPageInfo;
  initialTotalCount: number;
  onTotalCountChange: (count: number) => void;
};

export function MasterCardsClient({
  masterId,
  deckName,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
  onTotalCountChange,
}: Props) {
  const t = useTranslations("Cards");
  const tCardgroups = useTranslations("Cardgroups");
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
  } = useMasterCardsConnection({
    masterId,
    searchQuery: search.query,
    initialEdges,
    initialPageInfo,
    initialTotalCount,
    fetchMoreErrorMessage: t("fetchMoreFailed"),
  });

  const {
    createCard,
    updateCard,
    deleteRow,
    deleteCards,
    creating,
    updating,
    bulkDeleting,
    createError,
    updateError,
    bulkDeleteError,
    deleteRowError,
    resetCreateCard,
  } = useMasterCardMutations({ masterId, queryVariables });

  const queryBannerError = getBackendErrorBanner(queryError);
  const bulkDeleteBannerError = getBackendErrorBanner(bulkDeleteError);
  // Raw per-row delete error → localized banner copy (server message or fallback).
  const deleteRowBannerError =
    deleteRowError !== null ? (getBackendErrorBanner(deleteRowError) ?? t("deleteError")) : null;

  // Report live total count upward so the page header stays in sync.
  useEffect(() => {
    onTotalCountChange(totalCount);
  }, [totalCount, onTotalCountChange]);

  const closeOtherRows = useCallback((exceptCardId: string) => {
    for (const [id, ref] of rowRefs.current.entries()) {
      if (id !== exceptCardId) ref.current?.close();
    }
  }, []);

  const editingCard = edges.find((edge) => edge.node.id === editingId)?.node;

  const openAddSheet = useCallback(() => {
    resetCreateCard();
    setCreateValidationError(null);
    setAddDirty(false);
    setAddOpen(true);
  }, [resetCreateCard]);

  const openBatchImport = useCallback(() => setBatchImportOpen(true), []);
  const closeBatchImport = useCallback(() => setBatchImportOpen(false), []);

  async function handleCreate(values: { front: string; back: string }) {
    resetCreateCard();
    setCreateValidationError(null);
    const outcome = await createCard(values);
    switch (outcome.status) {
      case "success":
        setAddDirty(false);
        setCreateValidationError(null);
        setAddOpen(false);
        break;
      case "validation":
        setCreateValidationError({ field: outcome.field, message: outcome.message });
        break;
      case "unexpected":
        setCreateValidationError({ field: "front", message: t("addFailed") });
        break;
      case "rejected":
        // The CardForm error banner surfaces the rejection via `createError`.
        break;
    }
  }

  async function handleBulkDelete() {
    const ids = Array.from(selection.selectedIds);
    try {
      await deleteCards(ids);
      selection.clearSelection();
    } catch (err) {
      console.error("[MasterCardsClient] bulk delete rejection", {
        name: err instanceof Error ? err.name : "unknown",
        masterId,
        ids,
      });
    }
  }

  async function handleUpdate(id: string, values: { front: string; back: string }) {
    setRowValidationError(null);
    const outcome = await updateCard(id, values);
    switch (outcome.status) {
      case "success":
        setEditingId(null);
        break;
      case "validation":
        setRowValidationError({ field: outcome.field, message: outcome.message });
        break;
      case "unexpected":
        setRowValidationError({ field: "front", message: t("saveFailed") });
        break;
      case "rejected":
        // The edit-sheet error banner surfaces the rejection via `updateError`.
        break;
    }
  }

  return (
    <div className="space-y-3">
      {queryBannerError && (
        <ErrorBanner data-testid="cards-query-error">{queryBannerError}</ErrorBanner>
      )}
      {deleteRowBannerError && (
        <ErrorBanner data-testid="cards-delete-error">{deleteRowBannerError}</ErrorBanner>
      )}
      {bulkDeleteBannerError && (
        <ErrorBanner data-testid="cards-bulk-delete-error">{bulkDeleteBannerError}</ErrorBanner>
      )}

      <section>
        {/* Inline toolbar: right-aligned Add card and Batch import buttons. */}
        <div className="mb-3 flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={openAddSheet}
            data-testid="master-add-card"
          >
            {t("addCard")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={openBatchImport}
            data-testid="master-batch-import"
          >
            {tCardgroups("batchImport")}
          </Button>
        </div>

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
                    onDelete={() => deleteRow(card.id, t("cardDeleted"))}
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

        <FormSheet
          title={deckName}
          open={batchImportOpen}
          onOpenChange={setBatchImportOpen}
          size="lg"
        >
          <MasterBatchImportForm
            masterId={masterId}
            deckName={deckName}
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
