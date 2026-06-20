"use client";

import { useTranslations } from "next-intl";
import type { ReactNode, RefObject } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { BulkActionBar } from "@/components/cardgroups/bulk-action-bar";
import { CardFetchMoreError } from "@/components/cardgroups/card-fetch-more-error";
import { CardForm } from "@/components/cardgroups/card-form";
import { CardRow } from "@/components/cardgroups/card-row";
import { CardSearchInput } from "@/components/cardgroups/card-search-input";
import type { SwipeableRowHandle } from "@/components/cardgroups/swipeable-row";
import { SearchTakeoverBar } from "@/components/search/search-takeover-bar";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/components/ui/error-banner";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import type { AdminMasterCardsConnectionQuery } from "@/generated/graphql";
import { useBulkSelection } from "@/hooks/use-bulk-selection";
import { useHeaderTakeoverSearch } from "@/hooks/use-header-takeover-search";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { type AddMasterCardDetail, FLAMINGO_EVENT } from "@/lib/events/flamingo-events";
import { useCardSheetForm } from "@/lib/forms/use-sheet-form";
import { MasterBatchImportForm } from "./master-batch-import-form";
import { useMasterCardMutations } from "./use-master-card-mutations";
import { useMasterCardsConnection } from "./use-master-cards-connection";

type Connection = AdminMasterCardsConnectionQuery["adminMasterCardsConnection"];
export type MasterCardEdge = Connection["edges"][number];
export type MasterCardConnectionPageInfo = Connection["pageInfo"];

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
  masterId: string;
  deckName: string;
  initialEdges: MasterCardEdge[];
  initialPageInfo: MasterCardConnectionPageInfo;
  initialTotalCount: number;
  /**
   * Page-level header rendered above the search box. Pass a render function to
   * receive the live `totalCount` (sourced from the Apollo cache, kept in sync
   * with create/delete/fetchMore) plus the add-card and batch-import openers,
   * or a plain `ReactNode` to render as-is. Omitted → no header is rendered.
   */
  sectionHeader?: ReactNode | ((args: SectionHeaderArgs) => ReactNode);
};

export function MasterCardsClient({
  masterId,
  deckName,
  initialEdges,
  initialPageInfo,
  initialTotalCount,
  sectionHeader,
}: Props) {
  const t = useTranslations("Cards");
  const [batchImportOpen, setBatchImportOpen] = useState(false);
  // Per-card SwipeableRow handles; lets a row close any half-open sibling.
  const rowRefs = useRef<Map<string, RefObject<SwipeableRowHandle | null>>>(new Map());
  const selection = useBulkSelection<string>();
  const search = useHeaderTakeoverSearch();

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

  // The global header "+" (LogoDrawer) dispatches flamingo:add-master-card on
  // the master-edit route; claim it for this deck to open the add-card sheet.
  // Master cards have no separate-page create target, so there is no fallback
  // navigation to preventDefault against — the listener simply opens the sheet.
  useEffect(() => {
    function handleAddMasterCard(event: CustomEvent<AddMasterCardDetail>) {
      if (event.detail?.masterId !== masterId) return;
      event.preventDefault();
      sheet.openAddSheet();
    }
    window.addEventListener(FLAMINGO_EVENT.addMasterCard, handleAddMasterCard);
    return () => window.removeEventListener(FLAMINGO_EVENT.addMasterCard, handleAddMasterCard);
  }, [masterId, sheet.openAddSheet]);

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
          {/* Page header (deck title/badge/kebab + the desktop add/import toolbar)
            is supplied by the parent via the render-prop, so it receives the
            live totalCount and the add/import openers. On mobile the openers are
            reached through the global "+" header and the deck overflow menu. */}
          {typeof sectionHeader === "function"
            ? sectionHeader({
                totalCount,
                onAddCard: sheet.openAddSheet,
                onBatchImport: openBatchImport,
              })
            : sectionHeader}

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
              submit={sheet.handleCreate}
              submitting={creating}
              error={createError}
              validationError={sheet.createValidationError}
              onDirty={sheet.markAddDirty}
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

          <div ref={sentinelRef} aria-hidden="true" data-testid="cards-sentinel" />
          {fetchMoreError && (
            <CardFetchMoreError message={fetchMoreError} onRetry={retryFetchMore} />
          )}
          {!fetchMoreError && fetchingMore && pageInfo.hasNextPage && (
            <p className="mt-3 text-center text-xs text-muted-foreground">{t("loadingMore")}</p>
          )}
        </section>
      </div>
    </>
  );
}
