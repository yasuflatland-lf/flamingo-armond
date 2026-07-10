"use client";

import {
  CreateCardMutation,
  DeleteCardMutation,
  DeleteCardsMutation,
  UpdateCardMutation,
} from "@/app/cardgroups/queries";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import {
  defineEntityCardMutationsConfig,
  useEntityCardMutations,
} from "@/lib/cards/use-entity-card-mutations";
import { cardsDefaultVars } from "./queries";

// Static config: documents, typenames, and the typed classifier/builder
// callbacks. A module constant so the generic hook's useCallback memoization
// stays stable across renders.
const CARD_MUTATIONS_CONFIG = defineEntityCardMutationsConfig({
  scope: "[useCardMutations]",
  ownerLogKey: "cardgroupId",
  createOpName: "createCard",
  updateOpName: "updateCard",
  connectionDocument: CardsByCardgroupConnectionDocument,
  connectionField: "cardsByCardgroupConnection",
  edgeTypename: "CardEdge",
  entityTypename: "Card",
  defaultVars: cardsDefaultVars,
  createDocument: CreateCardMutation,
  buildCreateVariables: (cardgroupId, values) => ({
    input: { cardgroupId, front: values.front, back: values.back },
  }),
  classifyCreate: (data) => {
    const payload = data?.createCard;
    if (payload?.__typename === "CreateCardSuccess") {
      return { kind: "success", node: payload.card };
    }
    if (payload?.__typename === "CardDuplicateFrontError") {
      return { kind: "duplicate", message: payload.message };
    }
    return {
      kind: "unknown",
      typename: (payload as { __typename?: string } | null | undefined)?.__typename ?? null,
    };
  },
  updateDocument: UpdateCardMutation,
  buildUpdateVariables: (id, values) => ({
    id,
    input: { front: values.front, back: values.back },
  }),
  classifyUpdate: (data) => {
    const payload = data?.updateCard;
    if (payload?.__typename === "UpdateCardSuccess") {
      return { kind: "success" };
    }
    if (payload?.__typename === "InputValidationError") {
      return { kind: "validation", field: payload.field, message: payload.message };
    }
    return {
      kind: "unknown",
      typename: (payload as { __typename?: string } | null | undefined)?.__typename ?? null,
    };
  },
  deleteOneDocument: DeleteCardMutation,
  extractDeleteOne: (data) => Boolean(data?.deleteCard),
  deleteManyDocument: DeleteCardsMutation,
  extractDeleteManyCount: (data) => data?.deleteCards,
});

export interface UseCardMutationsInput {
  cardgroupId: string;
  /**
   * The connection's active query variables (search-filter-aware). MUST be the
   * same object the live `useQuery` is keyed on so cache reads/writes land on
   * the right entry under any active search filter.
   * See docs/pagination/optimistic-rollback-cache-key.md.
   */
  queryVariables: CardsByCardgroupConnectionQueryVariables;
}

/**
 * Owns the four card mutations (create / update / per-row delete / bulk delete)
 * for the `/cardgroups/[id]/cards` screen, delegating all logic to the generic
 * `useEntityCardMutations`. Thin wrapper: binds the card config and translates
 * the route's `cardgroupId` to the generic hook's `ownerId`.
 */
export function useCardMutations({ cardgroupId, queryVariables }: UseCardMutationsInput) {
  return useEntityCardMutations(CARD_MUTATIONS_CONFIG, {
    ownerId: cardgroupId,
    queryVariables,
  });
}
