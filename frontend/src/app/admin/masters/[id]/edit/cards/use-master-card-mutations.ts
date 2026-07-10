"use client";

import {
  AdminMasterCardsConnectionDocument,
  type AdminMasterCardsConnectionQueryVariables,
} from "@/generated/graphql";
import {
  defineEntityCardMutationsConfig,
  useEntityCardMutations,
} from "@/lib/cards/use-entity-card-mutations";
import {
  AdminCreateMasterCard,
  AdminDeleteMasterCard,
  AdminDeleteMasterCards,
  AdminUpdateMasterCard,
  masterCardsDefaultVars,
} from "./queries";

const MASTER_CARD_MUTATIONS_CONFIG = defineEntityCardMutationsConfig({
  scope: "[useMasterCardMutations]",
  ownerLogKey: "masterId",
  createOpName: "adminCreateMasterCard",
  updateOpName: "adminUpdateMasterCard",
  connectionDocument: AdminMasterCardsConnectionDocument,
  connectionField: "adminMasterCardsConnection",
  edgeTypename: "MasterCardEdge",
  entityTypename: "MasterCard",
  defaultVars: masterCardsDefaultVars,
  createDocument: AdminCreateMasterCard,
  buildCreateVariables: (masterId, values) => ({
    input: { masterCardgroupId: masterId, front: values.front, back: values.back },
  }),
  classifyCreate: (data) => {
    const payload = data?.adminCreateMasterCard;
    if (payload?.__typename === "CreateMasterCardSuccess") {
      return { kind: "success", node: payload.masterCard };
    }
    if (payload?.__typename === "MasterCardDuplicateFrontError") {
      return { kind: "duplicate", message: payload.message };
    }
    return {
      kind: "unknown",
      typename: (payload as { __typename?: string } | null | undefined)?.__typename ?? null,
    };
  },
  updateDocument: AdminUpdateMasterCard,
  buildUpdateVariables: (id, values) => ({
    id,
    input: { front: values.front, back: values.back },
  }),
  classifyUpdate: (data) => {
    const payload = data?.adminUpdateMasterCard;
    if (payload?.__typename === "UpdateMasterCardSuccess") {
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
  deleteOneDocument: AdminDeleteMasterCard,
  extractDeleteOne: (data) => Boolean(data?.adminDeleteMasterCard),
  deleteManyDocument: AdminDeleteMasterCards,
  extractDeleteManyCount: (data) => data?.adminDeleteMasterCards,
});

export interface UseMasterCardMutationsInput {
  masterId: string;
  /**
   * The connection's active query variables (search-filter-aware). MUST be the
   * same object the live `useQuery` is keyed on so cache reads/writes land on
   * the right entry under any active search filter.
   * See docs/pagination/optimistic-rollback-cache-key.md.
   */
  queryVariables: AdminMasterCardsConnectionQueryVariables;
}

/**
 * Owns the four master-card mutations for the `/admin/masters/[id]/edit/cards`
 * screen, delegating all logic to the generic `useEntityCardMutations`. Thin
 * wrapper: binds the master-card config and translates the route's `masterId`
 * to the generic hook's `ownerId`.
 */
export function useMasterCardMutations({ masterId, queryVariables }: UseMasterCardMutationsInput) {
  return useEntityCardMutations(MASTER_CARD_MUTATIONS_CONFIG, {
    ownerId: masterId,
    queryVariables,
  });
}
