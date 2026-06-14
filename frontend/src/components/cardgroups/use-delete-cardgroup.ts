"use client";

import { useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { CARDGROUPS_DEFAULT_VARS, DeleteCardgroupMutation } from "@/app/cardgroups/queries";
import { MyCardgroupsConnectionDocument } from "@/generated/graphql";

/**
 * Owns the cardgroup delete mutation and its Connection-delete cache surgery:
 * filter the deleted edge out of `myCardgroupsConnection`, decrement
 * `totalCount` (clamped at 0 — the item may live on a page never fetched into
 * edges), then `evict` + `gc` the normalized entity (.claude/rules/pagination.md
 * "Connection delete"). The cache key is `CARDGROUPS_DEFAULT_VARS` so the
 * read/write match the SSR seed and client `useQuery` key
 * (docs/pagination/variables-shape-must-match.md).
 *
 * Unlike `useAdminUserMutations` (which re-throws so the caller's catch renders
 * the reason), this hook mirrors the header's existing contract: it swallows the
 * rejection (returns `false`) and exposes `useMutation`'s reactive `error` state
 * so the caller drives the banner via `getBackendErrorBanner(error)`. The caller
 * branches on the returned boolean for the success navigation.
 *
 * No `optimisticResponse`: a delete can fail with FORBIDDEN and Apollo does not
 * reliably roll back optimistic writes for typed GraphQL errors.
 */
export function useDeleteCardgroup() {
  const [runDeleteCardgroup, { loading: deleting, error }] = useMutation(DeleteCardgroupMutation);

  const deleteCardgroup = useCallback(
    async (id: string): Promise<boolean> => {
      const result = await runDeleteCardgroup({
        variables: { id },
        update(cache, { data }) {
          if (!data?.deleteCardgroup) return;

          const existingConnection = cache.readQuery({
            query: MyCardgroupsConnectionDocument,
            variables: CARDGROUPS_DEFAULT_VARS,
          });
          if (existingConnection) {
            cache.writeQuery({
              query: MyCardgroupsConnectionDocument,
              variables: CARDGROUPS_DEFAULT_VARS,
              data: {
                myCardgroupsConnection: {
                  ...existingConnection.myCardgroupsConnection,
                  edges: existingConnection.myCardgroupsConnection.edges.filter(
                    (edge) => edge.node.id !== id,
                  ),
                  totalCount: Math.max(0, existingConnection.myCardgroupsConnection.totalCount - 1),
                },
              },
            });
          }

          cache.evict({
            id: cache.identify({ __typename: "Cardgroup", id }),
          });
          cache.gc();
        },
      }).catch((err) => {
        console.error("[useDeleteCardgroup] delete rejection", err);
        return null;
      });

      return result?.data?.deleteCardgroup === true;
    },
    [runDeleteCardgroup],
  );

  return { deleteCardgroup, deleting, error };
}
