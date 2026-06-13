"use client";

import { useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { CARDGROUPS_DEFAULT_VARS } from "@/app/cardgroups/queries";
import { MyCardgroupsConnectionDocument } from "@/generated/graphql";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { ImportMasterCardgroupMutation } from "./queries";

/**
 * Discriminated outcome of an import-master attempt. The catalog client branches
 * on `status`: `success` shows a confirmation and the imported deck appears on
 * `/cardgroups` (the connection cache is updated inside this hook); `not_found`
 * and `rejected` surface a banner; `auth` surfaces a sign-in prompt.
 *
 * `not_found` collapses both "unknown id" and "exists but unpublished" — the
 * backend never discloses draft existence (see schema `MasterNotFoundError`).
 */
export type ImportMasterOutcome =
  | { status: "success"; cardgroupId: string; cardgroupName: string }
  | { status: "not_found"; message: string }
  | { status: "auth"; kind: "unauthenticated" | "forbidden" }
  // The mutation threw a non-auth transport error, or resolved with an
  // unparseable payload (unknown __typename / partial-response null bubble).
  | { status: "rejected" };

/**
 * Wraps `importMasterCardgroup` and, on success, prepends the new user-owned
 * cardgroup to the `myCardgroupsConnection` cache so it appears on `/cardgroups`
 * without a refetch.
 *
 * No `optimisticResponse`: the mutation can resolve with a typed `MasterNotFoundError`
 * or fail with UNAUTHENTICATED, and Apollo does not consistently roll back optimistic
 * writes for typed GraphQL errors. See .claude/rules/pagination.md § "Drop
 * optimisticResponse for mutations that can fail with typed GraphQL errors".
 */
export function useImportMaster() {
  const [importMaster, { loading }] = useMutation(ImportMasterCardgroupMutation, {
    update(cache, { data }) {
      // Narrow on __typename before touching .cardgroup so a MasterNotFoundError
      // or unknown variant does not mutate the cache.
      if (data?.importMasterCardgroup?.__typename !== "ImportMasterCardgroupSuccess") return;
      const created = data.importMasterCardgroup.cardgroup;

      // cache.modify is forbidden — use readQuery + writeQuery so a cold cache
      // (user landing on /catalog without having visited /cardgroups) is also
      // handled. CARDGROUPS_DEFAULT_VARS keeps the cache key in sync with the
      // /cardgroups SSR seed and client useQuery — any mismatch makes this write
      // invisible. See .claude/rules/pagination.md.
      const existingConnection = cache.readQuery({
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
      });
      const newEdge = {
        __typename: "CardgroupEdge" as const,
        cursor: created.id,
        node: created,
      };
      const nextConnection = existingConnection
        ? {
            ...existingConnection.myCardgroupsConnection,
            edges: [newEdge, ...existingConnection.myCardgroupsConnection.edges],
            totalCount: existingConnection.myCardgroupsConnection.totalCount + 1,
          }
        : {
            // Cold cache: build a minimal connection so the listing page renders
            // the new edge immediately when the user navigates there.
            __typename: "CardgroupConnection" as const,
            edges: [newEdge],
            pageInfo: {
              __typename: "PageInfo" as const,
              hasNextPage: false,
              hasPreviousPage: false,
              startCursor: created.id,
              endCursor: created.id,
            },
            totalCount: 1,
          };
      cache.writeQuery({
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
        data: { myCardgroupsConnection: nextConnection },
      });
    },
  });

  const importMasterCardgroup = useCallback(
    async (masterCardgroupId: string): Promise<ImportMasterOutcome> => {
      try {
        const result = await importMaster({ variables: { masterCardgroupId } });
        const payload = result.data?.importMasterCardgroup;
        // Capture typename before narrowing so the unknown-variant branch can log
        // it (TypeScript narrows to `never` after the known cases).
        const typename = payload?.__typename ?? null;
        if (payload?.__typename === "MasterNotFoundError") {
          return { status: "not_found", message: payload.message };
        }
        if (payload?.__typename === "ImportMasterCardgroupSuccess") {
          return {
            status: "success",
            cardgroupId: payload.cardgroup.id,
            cardgroupName: payload.cardgroup.name,
          };
        }
        // Null payload, partial-response null bubble, or a future union variant
        // the client was not regenerated against.
        console.warn("[useImportMaster] unexpected importMasterCardgroup payload", { typename });
        return { status: "rejected" };
      } catch (err) {
        const codes = liftGraphQLCodes(err);
        if (codes.includes("UNAUTHENTICATED")) return { status: "auth", kind: "unauthenticated" };
        if (codes.includes("FORBIDDEN")) return { status: "auth", kind: "forbidden" };
        // err.message is omitted — backend messages may echo user input.
        // codes is safe to log (fixed enum of GraphQL extension codes).
        console.warn("[useImportMaster] importMasterCardgroup rejected", {
          name: err instanceof Error ? err.name : "unknown",
          codes,
        });
        return { status: "rejected" };
      }
    },
    [importMaster],
  );

  return { importMasterCardgroup, loading };
}
