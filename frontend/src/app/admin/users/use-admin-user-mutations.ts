"use client";

import type { Reference } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { AdminDeleteUserMutation } from "./queries";

/**
 * Owns the admin delete-user mutation and its node-id cache eviction.
 *
 * `deleteUser` resolves `void` on success — after dropping the user from every
 * cached `users` connection variant (filtering by the normalized `node.id`) and
 * evicting the normalized entity. On failure it RE-THROWS the raw error:
 * `AdminUserProfileSheet`'s danger zone catches it and renders the reason via
 * `mutationAuthBanner`, so a typed-outcome shape would strip the error the sheet
 * needs. The cache-eviction half mirrors masters/cards/cardgroups — see
 * .claude/rules/pagination.md "Resolve an edge by node.id, never by edge.cursor"
 * and "Connection delete".
 *
 * No `optimisticResponse`: a delete can fail with FORBIDDEN and Apollo does not
 * reliably roll back optimistic writes for typed GraphQL errors.
 */
export function useAdminUserMutations() {
  const apolloClient = useApolloClient();
  const [runDeleteUser] = useMutation(AdminDeleteUserMutation);

  const deleteUser = useCallback(
    async (id: string): Promise<void> => {
      const result = await runDeleteUser({ variables: { id } });
      if (!result.data?.adminDeleteUser) {
        throw new Error("adminDeleteUser returned false");
      }
      apolloClient.cache.modify({
        fields: {
          users(existing, { readField }) {
            const connection = existing as {
              edges?: ReadonlyArray<{ node: Reference }>;
              totalCount?: number;
            };
            if (!connection.edges) return existing;
            // Filter by the normalized node id, NOT by `edge.cursor`: the backend
            // emits an opaque "v1:..." cursor that never equals the raw user id
            // (.claude/rules/pagination.md "Resolve an edge by node.id, never by
            // edge.cursor"). Matches masters/cards/cardgroups.
            const edges = connection.edges.filter(
              (edge) => readField<string>("id", edge.node) !== id,
            );
            if (edges.length === connection.edges.length) return existing;
            return {
              ...connection,
              edges,
              totalCount: Math.max(0, (connection.totalCount ?? 0) - 1),
            };
          },
        },
      });
      const cacheId = apolloClient.cache.identify({ __typename: "User", id });
      if (cacheId) {
        apolloClient.cache.evict({ id: cacheId });
        apolloClient.cache.gc();
      }
    },
    [apolloClient, runDeleteUser],
  );

  return { deleteUser };
}
