"use client";

import { useApolloClient, useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { removeConnectionEdgeAcrossVariants } from "@/lib/apollo/connection-cache";
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
      removeConnectionEdgeAcrossVariants(apolloClient.cache, {
        connectionField: "users",
        entityTypename: "User",
        id,
      });
    },
    [apolloClient, runDeleteUser],
  );

  return { deleteUser };
}
