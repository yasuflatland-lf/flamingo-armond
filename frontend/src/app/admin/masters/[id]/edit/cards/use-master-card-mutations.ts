"use client";

import { useApolloClient, useMutation } from "@apollo/client/react";
import { useCallback, useState } from "react";
import {
  AdminMasterCardsConnectionDocument,
  type AdminMasterCardsConnectionQuery,
  type AdminMasterCardsConnectionQueryVariables,
} from "@/generated/graphql";
import { useUndoDelete } from "@/lib/undo-delete";
import {
  AdminCreateMasterCard,
  AdminDeleteMasterCard,
  AdminDeleteMasterCards,
  AdminUpdateMasterCard,
  masterCardsDefaultVars,
} from "./queries";

type MasterCardNode =
  AdminMasterCardsConnectionQuery["adminMasterCardsConnection"]["edges"][number]["node"];

/** Outcome of a create attempt; the client maps it to validation / sheet / banner state. */
type CreateCardOutcome =
  | { status: "success" }
  | { status: "validation"; field: string; message: string }
  | { status: "unexpected" }
  | { status: "rejected" };

/** Outcome of an update attempt; the client maps it to the inline row validation error. */
type UpdateCardOutcome =
  | { status: "success" }
  | { status: "validation"; field: string; message: string }
  | { status: "unexpected" }
  | { status: "rejected" };

function cardMatchesSearch(card: MasterCardNode, searchValue: string): boolean {
  const normalized = searchValue.trim().toLowerCase();
  if (normalized === "") return true;
  return (
    card.front.toLowerCase().includes(normalized) || card.back.toLowerCase().includes(normalized)
  );
}

export interface UseMasterCardMutationsInput {
  masterId: string;
  /**
   * The connection's active query variables (search-filter-aware). MUST be the
   * same object the live `useQuery` is keyed on so cache reads/writes land on the
   * right entry under any active search filter.
   * See docs/pagination/optimistic-rollback-cache-key.md.
   */
  queryVariables: AdminMasterCardsConnectionQueryVariables;
}

/**
 * Owns the four master card mutations (create / update / per-row delete / bulk delete)
 * and every Apollo cache write they perform. The client maps the returned typed
 * outcomes to sheet navigation, validation, and banner state. Mirrors the
 * thin-outcome-hook shape of `useMasterMutations` / `useAdminUserMutations`.
 *
 * No `optimisticResponse`: typed errors (MasterCardDuplicateFrontError,
 * InputValidationError, UNAUTHENTICATED, FORBIDDEN) can fail these mutations and
 * Apollo does not reliably roll back optimistic writes for typed GraphQL errors
 * — see .claude/rules/pagination.md.
 */
export function useMasterCardMutations({ masterId, queryVariables }: UseMasterCardMutationsInput) {
  const apollo = useApolloClient();
  const { scheduleDelete } = useUndoDelete();

  const [
    createMasterCardMutation,
    { loading: creating, error: createError, reset: resetCreateCard },
  ] = useMutation(AdminCreateMasterCard);
  // Updates propagate automatically via Apollo cache normalization (MasterCard has id).
  const [updateMasterCardMutation, { loading: updating, error: updateError }] =
    useMutation(AdminUpdateMasterCard);
  // scheduleDelete (5s undo window) fires the per-row mutation imperatively.
  const [deleteMasterCardMutation] = useMutation(AdminDeleteMasterCard);

  const [deleteCardsMutation, { error: bulkDeleteError, loading: bulkDeleting }] = useMutation(
    AdminDeleteMasterCards,
    {
      // queryVariables matches the live cache entry under any active search filter.
      // See docs/pagination/optimistic-rollback-cache-key.md.
      update(cache, { data: bulkData }, { variables: mutationVars }) {
        const ids = mutationVars?.ids as string[] | undefined;
        if (!ids) return;
        if (bulkData?.adminDeleteMasterCards == null) return;
        const deletedCount = bulkData.adminDeleteMasterCards;
        if (deletedCount === 0) return;
        const existing = cache.readQuery({
          query: AdminMasterCardsConnectionDocument,
          variables: queryVariables,
        });
        if (existing) {
          const next = existing.adminMasterCardsConnection;
          cache.writeQuery({
            query: AdminMasterCardsConnectionDocument,
            variables: queryVariables,
            data: {
              adminMasterCardsConnection: {
                ...next,
                edges: next.edges.filter((edge) => !ids.includes(edge.node.id)),
                totalCount: Math.max(0, next.totalCount - deletedCount),
              },
            },
          });
        }
        for (const id of ids) {
          cache.evict({ id: cache.identify({ __typename: "MasterCard", id }) });
        }
        cache.gc();
      },
    },
  );

  // Per-row delete commit error — surfaced after the 5s undo window elapses and
  // the DELETE mutation rejects. Stored as the raw error so the client derives
  // the localized banner copy; persists across mutation calls. See
  // docs/pagination/do-not-reuse-mutation-error-state.md.
  const [deleteRowError, setDeleteRowError] = useState<unknown>(null);

  // Write a freshly-created master card to the connection cached under `variables`.
  const writeCreatedMasterCardToConnection = useCallback(
    (card: MasterCardNode, variables: AdminMasterCardsConnectionQueryVariables) => {
      const existing = apollo.readQuery({
        query: AdminMasterCardsConnectionDocument,
        variables,
      });
      if (!existing) return;

      const next = existing.adminMasterCardsConnection;
      if (next.edges.some((edge) => edge.node.id === card.id)) return;

      apollo.writeQuery({
        query: AdminMasterCardsConnectionDocument,
        variables,
        data: {
          adminMasterCardsConnection: {
            ...next,
            edges: [
              { __typename: "MasterCardEdge" as const, cursor: card.id, node: card },
              ...next.edges,
            ],
            totalCount: next.totalCount + 1,
          },
        },
      });
    },
    [apollo],
  );

  const createCard = useCallback(
    async (values: { front: string; back: string }): Promise<CreateCardOutcome> => {
      const result = await createMasterCardMutation({
        variables: {
          input: { masterCardgroupId: masterId, front: values.front, back: values.back },
        },
      }).catch((err) => {
        console.error("[useMasterCardMutations] create rejection", {
          name: err instanceof Error ? err.name : "unknown",
          masterId,
        });
        return null;
      });
      if (!result) return { status: "rejected" };

      const payload = result.data?.adminCreateMasterCard;
      if (payload?.__typename === "CreateMasterCardSuccess") {
        writeCreatedMasterCardToConnection(payload.masterCard, masterCardsDefaultVars(masterId));
        if (
          typeof queryVariables.search === "string" &&
          cardMatchesSearch(payload.masterCard, queryVariables.search)
        ) {
          writeCreatedMasterCardToConnection(payload.masterCard, queryVariables);
        }
        return { status: "success" };
      }
      if (payload?.__typename === "MasterCardDuplicateFrontError") {
        return { status: "validation", field: "front", message: payload.message };
      }
      const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
      console.warn("[useMasterCardMutations] unexpected adminCreateMasterCard payload", {
        typename: unknownPayload?.__typename ?? null,
        masterId,
      });
      return { status: "unexpected" };
    },
    [masterId, createMasterCardMutation, queryVariables, writeCreatedMasterCardToConnection],
  );

  const updateCard = useCallback(
    async (id: string, values: { front: string; back: string }): Promise<UpdateCardOutcome> => {
      const result = await updateMasterCardMutation({
        variables: { id, input: { front: values.front, back: values.back } },
      }).catch((err) => {
        console.error("[useMasterCardMutations] update rejection", {
          name: err instanceof Error ? err.name : "unknown",
          masterId,
          cardId: id,
        });
        return null;
      });
      if (!result) return { status: "rejected" };

      const payload = result.data?.adminUpdateMasterCard;
      if (payload?.__typename === "UpdateMasterCardSuccess") {
        return { status: "success" };
      }
      if (payload?.__typename === "InputValidationError") {
        return { status: "validation", field: payload.field, message: payload.message };
      }
      const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
      console.warn("[useMasterCardMutations] unexpected adminUpdateMasterCard payload", {
        typename: unknownPayload?.__typename ?? null,
        cardId: id,
        masterId,
      });
      return { status: "unexpected" };
    },
    [masterId, updateMasterCardMutation],
  );

  // Per-row delete: snapshot, optimistic drop, schedule DELETE with a 5s undo
  // window. See docs/pagination/optimistic-rollback-cache-key.md for the
  // queryVariables rule. `label` is the caller-supplied (localized) toast title.
  const deleteRow = useCallback(
    (cardId: string, label: string) => {
      const snapshot = apollo.readQuery({
        query: AdminMasterCardsConnectionDocument,
        variables: queryVariables,
      });
      if (snapshot) {
        const next = snapshot.adminMasterCardsConnection;
        apollo.writeQuery({
          query: AdminMasterCardsConnectionDocument,
          variables: queryVariables,
          data: {
            adminMasterCardsConnection: {
              ...next,
              edges: next.edges.filter((edge) => edge.node.id !== cardId),
              totalCount: Math.max(0, next.totalCount - 1),
            },
          },
        });
      }
      scheduleDelete({
        id: cardId,
        label,
        optimisticRollback: () => {
          if (snapshot !== null) {
            apollo.writeQuery({
              query: AdminMasterCardsConnectionDocument,
              variables: queryVariables,
              data: snapshot,
            });
          }
          setDeleteRowError(null);
        },
        commitDelete: async () => {
          setDeleteRowError(null);
          const result = await deleteMasterCardMutation({ variables: { id: cardId } });
          if (result.data?.adminDeleteMasterCard) {
            apollo.cache.evict({
              id: apollo.cache.identify({ __typename: "MasterCard", id: cardId }),
            });
            apollo.cache.gc();
          }
        },
        onCommitFailed: (err) => {
          setDeleteRowError(err);
        },
      });
    },
    [apollo, deleteMasterCardMutation, queryVariables, scheduleDelete],
  );

  const deleteCards = useCallback(
    (ids: string[]) => deleteCardsMutation({ variables: { ids } }),
    [deleteCardsMutation],
  );

  return {
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
  };
}
