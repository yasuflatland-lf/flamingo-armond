"use client";

import { useApolloClient, useMutation } from "@apollo/client/react";
import { useCallback, useState } from "react";
import {
  CreateCardMutation,
  DeleteCardMutation,
  DeleteCardsMutation,
  UpdateCardMutation,
} from "@/app/cardgroups/queries";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import { appendConnectionEdge, removeConnectionEdges } from "@/lib/apollo/connection-cache";
import { useUndoDelete } from "@/lib/undo-delete";
import { cardsDefaultVars } from "./queries";

type CardNode =
  CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"]["edges"][number]["node"];

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

function cardMatchesSearch(card: CardNode, searchValue: string): boolean {
  const normalized = searchValue.trim().toLowerCase();
  if (normalized === "") return true;
  return (
    card.front.toLowerCase().includes(normalized) || card.back.toLowerCase().includes(normalized)
  );
}

export interface UseCardMutationsInput {
  cardgroupId: string;
  /**
   * The connection's active query variables (search-filter-aware). MUST be the
   * same object the live `useQuery` is keyed on so cache reads/writes land on the
   * right entry under any active search filter.
   * See docs/pagination/optimistic-rollback-cache-key.md.
   */
  queryVariables: CardsByCardgroupConnectionQueryVariables;
}

/**
 * Owns the four card mutations (create / update / per-row delete / bulk delete)
 * and every Apollo cache write they perform. The client maps the returned typed
 * outcomes to sheet navigation, validation, and banner state. Mirrors the
 * thin-outcome-hook shape of `useMasterMutations` / `useAdminUserMutations`.
 *
 * No `optimisticResponse`: typed errors (CardDuplicateFrontError,
 * InputValidationError, UNAUTHENTICATED, FORBIDDEN) can fail these mutations and
 * Apollo does not reliably roll back optimistic writes for typed GraphQL errors
 * — see .claude/rules/pagination.md.
 */
export function useCardMutations({ cardgroupId, queryVariables }: UseCardMutationsInput) {
  const apollo = useApolloClient();
  const { scheduleDelete } = useUndoDelete();

  const [createCardMutation, { loading: creating, error: createError, reset: resetCreateCard }] =
    useMutation(CreateCardMutation);
  // Updates propagate automatically via Apollo cache normalization (Card has id).
  const [updateCardMutation, { loading: updating, error: updateError }] =
    useMutation(UpdateCardMutation);
  // scheduleDelete (5s undo window) fires the per-row mutation imperatively.
  const [deleteCardMutation] = useMutation(DeleteCardMutation);

  const [deleteCardsMutation, { error: bulkDeleteError, loading: bulkDeleting }] = useMutation(
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
        removeConnectionEdges(cache, {
          document: CardsByCardgroupConnectionDocument,
          variables: queryVariables,
          connectionField: "cardsByCardgroupConnection",
          entityTypename: "Card",
          ids,
          deletedCount,
        });
      },
    },
  );

  // Per-row delete commit error — surfaced after the 5s undo window elapses and
  // the DELETE mutation rejects. Stored as the raw error so the client derives
  // the localized banner copy; persists across mutation calls. See
  // docs/pagination/do-not-reuse-mutation-error-state.md.
  const [deleteRowError, setDeleteRowError] = useState<unknown>(null);

  // Write a freshly-created card to the connection cached under `variables`.
  // No `buildColdConnection`: on a cold cache this no-ops and the page's own
  // `useQuery` owns populating the connection.
  const writeCreatedCardToConnection = useCallback(
    (card: CardNode, variables: CardsByCardgroupConnectionQueryVariables) => {
      appendConnectionEdge(apollo.cache, {
        document: CardsByCardgroupConnectionDocument,
        variables,
        connectionField: "cardsByCardgroupConnection",
        edgeTypename: "CardEdge",
        node: card,
      });
    },
    [apollo],
  );

  const createCard = useCallback(
    async (values: { front: string; back: string }): Promise<CreateCardOutcome> => {
      const result = await createCardMutation({
        variables: { input: { cardgroupId, front: values.front, back: values.back } },
      }).catch((err) => {
        console.error("[useCardMutations] create rejection", {
          name: err instanceof Error ? err.name : "unknown",
          cardgroupId,
        });
        return null;
      });
      if (!result) return { status: "rejected" };

      const payload = result.data?.createCard;
      if (payload?.__typename === "CreateCardSuccess") {
        writeCreatedCardToConnection(payload.card, cardsDefaultVars(cardgroupId));
        if (
          typeof queryVariables.search === "string" &&
          cardMatchesSearch(payload.card, queryVariables.search)
        ) {
          writeCreatedCardToConnection(payload.card, queryVariables);
        }
        return { status: "success" };
      }
      if (payload?.__typename === "CardDuplicateFrontError") {
        return { status: "validation", field: "front", message: payload.message };
      }
      const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
      console.warn("[useCardMutations] unexpected createCard payload", {
        typename: unknownPayload?.__typename ?? null,
        cardgroupId,
      });
      return { status: "unexpected" };
    },
    [cardgroupId, createCardMutation, queryVariables, writeCreatedCardToConnection],
  );

  const updateCard = useCallback(
    async (id: string, values: { front: string; back: string }): Promise<UpdateCardOutcome> => {
      const result = await updateCardMutation({
        variables: { id, input: { front: values.front, back: values.back } },
      }).catch((err) => {
        console.error("[useCardMutations] update rejection", {
          name: err instanceof Error ? err.name : "unknown",
          cardgroupId,
          cardId: id,
        });
        return null;
      });
      if (!result) return { status: "rejected" };

      const payload = result.data?.updateCard;
      if (payload?.__typename === "UpdateCardSuccess") {
        return { status: "success" };
      }
      if (payload?.__typename === "InputValidationError") {
        return { status: "validation", field: payload.field, message: payload.message };
      }
      const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
      console.warn("[useCardMutations] unexpected updateCard payload", {
        typename: unknownPayload?.__typename ?? null,
        cardId: id,
        cardgroupId,
      });
      return { status: "unexpected" };
    },
    [cardgroupId, updateCardMutation],
  );

  // Per-row delete: snapshot, optimistic drop, schedule DELETE with a 5s undo
  // window. See docs/pagination/optimistic-rollback-cache-key.md for the
  // queryVariables rule. `label` is the caller-supplied (localized) toast title.
  const deleteRow = useCallback(
    (cardId: string, label: string) => {
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
        label,
        optimisticRollback: () => {
          if (snapshot !== null) {
            apollo.writeQuery({
              query: CardsByCardgroupConnectionDocument,
              variables: queryVariables,
              data: snapshot,
            });
          }
          setDeleteRowError(null);
        },
        commitDelete: async () => {
          setDeleteRowError(null);
          const result = await deleteCardMutation({ variables: { id: cardId } });
          if (result.data?.deleteCard) {
            apollo.cache.evict({ id: apollo.cache.identify({ __typename: "Card", id: cardId }) });
            apollo.cache.gc();
          }
        },
        onCommitFailed: (err) => {
          setDeleteRowError(err);
        },
      });
    },
    [apollo, deleteCardMutation, queryVariables, scheduleDelete],
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
