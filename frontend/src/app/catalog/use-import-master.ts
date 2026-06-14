"use client";

import { useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { prependMyCardgroupEdge } from "@/app/cardgroups/cache";
import { classifyMutationAuthError } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { ImportMasterCardgroupMutation } from "./queries";

/**
 * Discriminated outcome of an import-master attempt. The catalog client branches
 * on `status`: `success` shows a confirmation and the imported deck appears on
 * `/cardgroups` (the connection cache is updated inside this hook); `not_found`
 * and `rejected` surface a banner; `auth` surfaces a sign-in prompt.
 *
 * `not_found` carries no payload: it collapses both "unknown id" and "exists but
 * unpublished", and the backend's `MasterNotFoundError.message` must never reach
 * the user (it could disclose draft existence or echo user input). The client
 * renders its own localized copy, so the server message is intentionally dropped
 * at this boundary rather than threaded through as a dead field.
 *
 * Unlike the sibling `CreateCardgroupOutcome` (which splits `unexpected` from
 * `rejected`), the resolved-but-unparseable and transport-error cases are
 * collapsed into a single `rejected` here, because no catalog caller
 * distinguishes them — both render the same generic error banner.
 */
export type ImportMasterOutcome =
  | { status: "success"; cardgroupId: string; cardgroupName: string }
  | { status: "not_found" }
  // `auth.kind` mirrors the two GraphQL auth codes. `importMasterCardgroup`
  // currently only emits UNAUTHENTICATED; `forbidden` is handled defensively so a
  // future authorization rule degrades to the sign-in prompt, not the generic
  // error banner.
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
      prependMyCardgroupEdge(cache, data.importMasterCardgroup.cardgroup);
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
          // The backend message is deliberately discarded (non-disclosure); the
          // client surfaces its own localized copy.
          return { status: "not_found" };
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
        const authKind = classifyMutationAuthError(err);
        if (authKind !== "other") return { status: "auth", kind: authKind };
        // err.message is omitted — backend messages may echo user input.
        // codes is safe to log (fixed enum of GraphQL extension codes).
        console.warn("[useImportMaster] importMasterCardgroup rejected", {
          name: err instanceof Error ? err.name : "unknown",
          codes: liftGraphQLCodes(err),
        });
        return { status: "rejected" };
      }
    },
    [importMaster],
  );

  return { importMasterCardgroup, loading };
}
