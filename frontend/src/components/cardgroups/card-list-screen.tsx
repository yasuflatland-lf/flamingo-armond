"use client";

import { Check } from "lucide-react";
import { useTranslations } from "next-intl";
import type { ReactNode, RefObject } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { BulkActionBar } from "@/components/cardgroups/bulk-action-bar";
import { CardForm } from "@/components/cardgroups/card-form";
import { CardRow } from "@/components/cardgroups/card-row";
import { CardSearchInput } from "@/components/cardgroups/card-search-input";
import type { SwipeableRowHandle } from "@/components/cardgroups/swipeable-row";
import { ConnectionListFooter } from "@/components/layout/connection-list-footer";
import { SearchTakeoverBar } from "@/components/search/search-takeover-bar";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import { useBulkSelection } from "@/hooks/use-bulk-selection";
import type { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import type {
  CardFormValues,
  CreateCardOutcome,
  UpdateCardOutcome,
} from "@/lib/cards/card-mutation-outcomes";
import {
  FLAMINGO_EVENT,
  type FlamingoEventName,
  subscribeFlamingo,
} from "@/lib/events/flamingo-events";
import { useCardSheetForm } from "@/lib/forms/use-sheet-form";

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
  addedCount,
}: {
  submit: (values: { front: string; back: string }) => Promise<void>;
  submitting: boolean;
  error: unknown;
  validationError: { field: string; message: string } | null;
  onDirty: () => void;
  addedCount: number;
}) {
  const close = useFormSheetClose();
  const t = useTranslations("Cards");

  return (
    <div onInput={onDirty} className="space-y-3">
      {addedCount > 0 ? (
        <p
          className="flex items-center gap-1.5 text-sm text-success"
          data-testid="add-card-added-count"
        >
          <Check aria-hidden="true" className="h-4 w-4" />
          {t("addedCount", { count: addedCount })}
        </p>
      ) : null}
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

export type SectionHeaderArgs = {
  totalCount: number;
  onAddCard: () => void;
  onBatchImport: () => void;
};

/** A connection edge as the screen consumes it (CardRow needs only id/front/back). */
type CardListEdge = { cursor: string; node: { id: string; front: string; back: string } };

/** The connection-hook result the screen reads (a structural subset of UseConnectionPaginationResult). */
interface CardListConnection {
  edges: CardListEdge[];
  pageInfo: { hasNextPage: boolean };
  totalCount: number;
  fetchingMore: boolean;
  fetchMoreError: string | null;
  retryFetchMore: () => void;
  sentinelRef: RefObject<HTMLDivElement | null>;
  queryError: unknown;
}

/** The card mutations the screen drives (the shape of useEntityCardMutations' return). */
interface CardListMutations {
  createCard: (values: CardFormValues) => Promise<CreateCardOutcome>;
  updateCard: (id: string, values: CardFormValues) => Promise<UpdateCardOutcome>;
  deleteRow: (cardId: string, label: string) => void;
  deleteCards: (ids: string[]) => Promise<unknown>;
  creating: boolean;
  updating: boolean;
  bulkDeleting: boolean;
  createError: unknown;
  updateError: unknown;
  bulkDeleteError: unknown;
  deleteRowError: unknown;
  resetCreateCard: () => void;
}

export interface CardListScreenProps {
  /** Header-takeover search state (owned by the wrapper; drives the search UI). */
  search: ReturnType<typeof useHeaderTakeoverSearch>;
  /** Pagination/connection result from the entity's connection hook. */
  connection: CardListConnection;
  /** The entity's card mutations (from useEntityCardMutations via the wrapper). */
  mutations: CardListMutations;
  /** Owner id (cardgroupId / masterId) — for the add-card event match and bulk-delete log. */
  ownerId: string;
  /**
   * Global add-card event wiring. `name` is the FLAMINGO_EVENT key; `matches`
   * stays a per-wrapper closure (it reads the entity-specific `detail` field).
   */
  addCardEvent: { name: FlamingoEventName; matches: (detail: unknown) => boolean };
  /** Bulk-delete rejection log shape: scope tag + the owner-id key name. */
  bulkDeleteLog: { scope: string; ownerKey: string };
  /** Optional section header (render-prop receiving live totalCount, or a node). */
  sectionHeader?: ReactNode | ((args: SectionHeaderArgs) => ReactNode);
  /**
   * Header rendered when `sectionHeader === undefined` (cards: the count h2;
   * master: omitted, so an undefined header renders nothing).
   */
  defaultHeader?: (args: SectionHeaderArgs) => ReactNode;
  /** Batch-import sheet title (cards: ellipsis span; master: plain deck name). */
  importSheetTitle: ReactNode;
  /** Batch-import form slot; receives the shared close handlers. */
  renderImportForm: (args: { onImported: () => void; onCancel: () => void }) => ReactNode;
}

/**
 * Shared orchestration for the cards (`/cardgroups/[id]/cards`) and master-cards
 * (`/admin/masters/[id]/edit/cards`) list screens. Owns the presentational state
 * (sheets, editing row, row refs, bulk selection), the add/update/bulk-delete
 * handlers, and the full render (search bar, error banners, list, three
 * FormSheets, and the ConnectionListFooter). The two per-entity wrappers supply
 * the connection + mutation hook results and the three behavioral divergences
 * (default header, add-card event, batch-import sheet) as props.
 */
export function CardListScreen({
  search,
  connection,
  mutations,
  ownerId,
  addCardEvent,
  bulkDeleteLog,
  sectionHeader,
  defaultHeader,
  importSheetTitle,
  renderImportForm,
}: CardListScreenProps) {
  const t = useTranslations("Cards");
  const tCommon = useTranslations("Common");

  const {
    edges,
    pageInfo,
    totalCount,
    fetchingMore,
    fetchMoreError,
    retryFetchMore,
    sentinelRef,
    queryError,
  } = connection;
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
  } = mutations;

  const [batchImportOpen, setBatchImportOpen] = useState(false);
  // Per-card SwipeableRow handles; lets a row close any half-open sibling.
  const rowRefs = useRef<Map<string, RefObject<SwipeableRowHandle | null>>>(new Map());
  const selection = useBulkSelection<string>();

  const sheet = useCardSheetForm({
    createCard,
    updateCard,
    resetCreateCard,
    addFailedMessage: t("addFailed"),
    saveFailedMessage: t("saveFailed"),
  });

  const queryBannerError = getBackendErrorBanner(queryError);
  const bulkDeleteBannerError = getBackendErrorBanner(bulkDeleteError);
  // Raw per-row delete error → localized banner copy (server message or fallback).
  const deleteRowBannerError =
    deleteRowError !== null ? (getBackendErrorBanner(deleteRowError) ?? t("deleteError")) : null;

  const closeOtherRows = useCallback((exceptCardId: string) => {
    for (const [id, ref] of rowRefs.current.entries()) {
      if (id !== exceptCardId) ref.current?.close();
    }
  }, []);

  const editingCard = edges.find((edge) => edge.node.id === sheet.editingId)?.node;

  const openBatchImport = useCallback(() => setBatchImportOpen(true), []);
  const closeBatchImport = useCallback(() => setBatchImportOpen(false), []);

  useEffect(() => {
    return subscribeFlamingo(addCardEvent.name, (detail, event) => {
      if (!addCardEvent.matches(detail)) return;
      event.preventDefault();
      sheet.openAddSheet();
    });
  }, [addCardEvent, sheet.openAddSheet]);

  // The header "+" Add menu opens batch import via a global event (mobile path);
  // the desktop split-button calls onBatchImport directly. Both end at openBatchImport.
  useEffect(() => {
    return subscribeFlamingo(FLAMINGO_EVENT.batchImport, (detail) => {
      if (detail?.ownerId !== ownerId) return;
      openBatchImport();
    });
  }, [ownerId, openBatchImport]);

  async function handleBulkDelete() {
    const ids = Array.from(selection.selectedIds);
    try {
      await deleteCards(ids);
      selection.clearSelection();
    } catch (err) {
      console.error(`${bulkDeleteLog.scope} bulk delete rejection`, {
        name: err instanceof Error ? err.name : "unknown",
        [bulkDeleteLog.ownerKey]: ownerId,
        ids,
      });
    }
  }

  const headerNode =
    sectionHeader === undefined
      ? defaultHeader?.({
          totalCount,
          onAddCard: sheet.openAddSheet,
          onBatchImport: openBatchImport,
        })
      : typeof sectionHeader === "function"
        ? sectionHeader({
            totalCount,
            onAddCard: sheet.openAddSheet,
            onBatchImport: openBatchImport,
          })
        : sectionHeader;

  return (
    <>
      <SearchTakeoverBar
        open={search.searchOpen}
        value={search.input}
        onChange={search.setInput}
        onClear={search.clear}
        onClose={search.closeSearch}
        placeholder={t("searchPlaceholder")}
        ariaLabel={t("searchAriaLabel")}
      />
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
          {headerNode}

          <CardSearchInput value={search.input} onChange={search.setInput} />

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
                      disabled={selection.count > 0 || sheet.editingId === card.id}
                      onSelectToggle={() => selection.toggleSelected(card.id)}
                      onEdit={() => {
                        closeOtherRows(card.id);
                        sheet.beginEdit(card.id);
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
            open={sheet.addOpen}
            onOpenChange={sheet.onAddOpenChange}
            submitting={creating}
            dirty={sheet.addDirty}
            confirmOnDismiss
          >
            <AddCardSheetContent
              key={`add-card-${sheet.createNonce}`}
              submit={sheet.handleCreate}
              submitting={creating}
              error={createError}
              validationError={sheet.createValidationError}
              onDirty={sheet.markAddDirty}
              addedCount={sheet.addedCount}
            />
          </FormSheet>

          <FormSheet
            title={importSheetTitle}
            open={batchImportOpen}
            onOpenChange={setBatchImportOpen}
            size="lg"
          >
            {renderImportForm({ onImported: closeBatchImport, onCancel: closeBatchImport })}
          </FormSheet>

          <FormSheet
            title={t("editCard")}
            open={editingCard !== undefined}
            onOpenChange={sheet.onEditOpenChange}
            submitting={updating}
            confirmOnDismiss={false}
          >
            {editingCard ? (
              <EditCardSheetContent
                card={editingCard}
                submit={(values) => sheet.handleUpdate(editingCard.id, values)}
                submitting={updating}
                error={updateError}
                validationError={sheet.rowValidationError}
              />
            ) : null}
          </FormSheet>

          <ConnectionListFooter
            sentinelRef={sentinelRef}
            fetchMoreError={fetchMoreError}
            onRetry={retryFetchMore}
            fetchingMore={fetchingMore}
            hasNextPage={pageInfo.hasNextPage}
            retryLabel={tCommon("retry")}
            loadingMoreLabel={t("loadingMore")}
            testIdPrefix="cards"
          />
        </section>
      </div>
    </>
  );
}
