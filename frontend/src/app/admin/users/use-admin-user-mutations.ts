"use client";

import { useApolloClient, useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { AdminDeleteUserMutation } from "./queries";

/**
 * Owns the admin delete-user mutation and its cursor-filter cache eviction.
 *
 * `deleteUser` resolves `void` on success — after dropping the user from every
 * cached `users` connection variant (filtering by `edge.cursor`, which equals the
 * user id by the schema's connection contract) and evicting the normalized entity.
 * On failure it RE-THROWS the raw error: `AdminUserProfileSheet`'s danger zone
 * catches it and renders the reason via `mutationAuthBanner`, so a typed-outcome
 * shape would strip the error the sheet needs. This is intentionally asymmetric
 * with masters' delete (node.id filter + swallow-into-outcome) — see issue #448
 * and .claude/rules/pagination.md "Connection delete".
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
          users(existing) {
            const connection = existing as {
              edges?: ReadonlyArray<{ cursor: string }>;
              totalCount?: number;
            };
            if (!connection.edges) return existing;
            // Filter by `edge.cursor`, NOT `readField("id", edge.node)`. This is the
            // deliberate exception to .claude/rules/pagination.md "Resolve an edge by
            // node.id, never by edge.cursor": the backend emits the raw user id as the
            // users-connection cursor (AdminUserEdge{Cursor: user.ID} in
            // backend/internal/usecase/admin_user.go — NOT cursor.Encode), so here
            // cursor === id. Masters/cards/cardgroups encode their cursors, so they must
            // use node.id; users intentionally differ (issue #448).
            const edges = connection.edges.filter((edge) => edge.cursor !== id);
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
