"use client";

import { useApolloClient, useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { MergeMasterCardgroupMutation } from "@/app/catalog/queries";
import { cardsDefaultVars } from "@/app/cardgroups/[id]/cards/queries";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import { classifyToAuthOutcome } from "@/lib/apollo/errors";

export type MergeFromCatalogOutcome =
  | { status: "success"; addedCount: number; updatedCount: number }
  | { status: "not_found" }
  | { status: "auth"; kind: "unauthenticated" | "forbidden" }
  | { status: "rejected" };

/**
 * Wraps `mergeMasterCardgroup` and, on success, refetches the cards connection
 * for the destination cardgroup so the edited deck immediately reflects the
 * merged cards.
 *
 * No `optimisticResponse`: the mutation can resolve to a typed
 * `MasterNotFoundError`, and Apollo does not reliably roll back optimistic
 * writes for typed GraphQL errors.
 */
export function useMergeFromCatalog(targetCardgroupId: string) {
  const apollo = useApolloClient();
  const refetchVariables = cardsDefaultVars(targetCardgroupId);

  const [mergeMasterCardgroup, { loading }] = useMutation(MergeMasterCardgroupMutation);

  const mergeFromCatalog = useCallback(
    async (masterCardgroupId: string): Promise<MergeFromCatalogOutcome> => {
      try {
        const result = await mergeMasterCardgroup({
          variables: {
            input: {
              masterCardgroupId,
              cardgroupId: targetCardgroupId,
            },
          },
        });
        const payload = result.data?.mergeMasterCardgroup;
        const typename = payload?.__typename ?? null;

        if (payload?.__typename === "MasterNotFoundError") {
          return { status: "not_found" };
        }
        if (payload?.__typename === "MergeMasterCardgroupSuccess") {
          await apollo.refetchQueries({
            include: [CardsByCardgroupConnectionDocument],
            onQueryUpdated: (observableQuery) => {
              const variables = observableQuery.options.variables as
                | CardsByCardgroupConnectionQueryVariables
                | undefined;
              if (
                !variables ||
                variables.cardgroupId !== refetchVariables.cardgroupId ||
                variables.first !== refetchVariables.first ||
                variables.search !== refetchVariables.search
              ) {
                return false;
              }
              return observableQuery.refetch(refetchVariables);
            },
          });
          return {
            status: "success",
            addedCount: payload.addedCount,
            updatedCount: payload.updatedCount,
          };
        }

        // Null payload, partial-response null bubble, or a future union variant.
        console.warn("[useMergeFromCatalog] unexpected mergeMasterCardgroup payload", {
          typename,
          targetCardgroupId,
        });
        return { status: "rejected" };
      } catch (err) {
        return classifyToAuthOutcome(err, "useMergeFromCatalog", "mergeMasterCardgroup", {
          targetCardgroupId,
        });
      }
    },
    [apollo, mergeMasterCardgroup, refetchVariables, targetCardgroupId],
  );

  return { mergeFromCatalog, loading };
}
