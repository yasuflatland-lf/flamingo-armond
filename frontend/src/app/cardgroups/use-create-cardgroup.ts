"use client";

import { useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { MyCardgroupsConnectionDocument } from "@/generated/graphql";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { CARDGROUPS_DEFAULT_VARS, CreateCardgroupMutation } from "./queries";

/**
 * Discriminated outcome of a create-cardgroup attempt. Callers branch on
 * `status`: the full-page route redirects on `success`; the in-list drawer
 * closes. The connection cache is updated inside the hook (see the mutation
 * `update` below), so neither caller needs to touch the cache.
 */
export type CreateCardgroupOutcome =
  | { status: "success"; cardgroupId: string }
  | { status: "validation"; field: string; message: string }
  | { status: "auth"; kind: "unauthenticated" | "forbidden" }
  // The mutation resolved with an unparseable payload (unknown __typename or a
  // partial-response null bubble).
  | { status: "unexpected" }
  // The mutation threw a non-auth transport/network error.
  | { status: "rejected" };

/**
 * Shared create-cardgroup mutation + Connection cache write. Used by both the
 * full-page `/cardgroups/new` route (onboarding / returnTo flows) and the
 * in-list FormSheet drawer so the cache-key handling and typed-error
 * classification live in exactly one place.
 *
 * No `optimisticResponse`: typed errors (FORBIDDEN, InputValidationError for a
 * duplicate name) can fail the mutation, and Apollo does not consistently roll
 * back optimistic writes for typed GraphQL errors. See
 * .claude/rules/pagination.md § "Drop optimisticResponse for mutations that can
 * fail with typed GraphQL errors".
 */
export function useCreateCardgroup() {
  const [createCardgroup, { loading }] = useMutation(CreateCardgroupMutation, {
    update(cache, { data }) {
      // Narrow on __typename before accessing .cardgroup so an InputValidationError
      // or unknown variant does not silently mutate the cache.
      if (data?.createCardgroup?.__typename !== "CreateCardgroupSuccess") return;
      const created = data.createCardgroup.cardgroup;

      // cache.modify is forbidden — use readQuery + writeQuery so cold-cache
      // entries are also handled correctly. CARDGROUPS_DEFAULT_VARS keeps the
      // cache key in sync with the SSR seed and the client useQuery — any
      // mismatch makes this write invisible. See .claude/rules/pagination.md.
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
            // Cold cache: build a minimal connection so the listing page can render
            // the new edge immediately when the user lands there.
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

  const create = useCallback(
    async (name: string): Promise<CreateCardgroupOutcome> => {
      try {
        const result = await createCardgroup({ variables: { input: { name } } });
        const payload = result.data?.createCardgroup;
        // Capture typename before narrowing so the unknown-variant branch still
        // has access to it (TypeScript narrows to `never` after the known cases).
        const typename = payload?.__typename ?? null;
        if (payload?.__typename === "InputValidationError") {
          return { status: "validation", field: payload.field, message: payload.message };
        }
        if (payload?.__typename === "CreateCardgroupSuccess") {
          return { status: "success", cardgroupId: payload.cardgroup.id };
        }
        // Null payload, partial-response null bubble, or a future union variant
        // the client was not regenerated against.
        console.warn("[useCreateCardgroup] unexpected createCardgroup payload", { typename });
        return { status: "unexpected" };
      } catch (err) {
        const codes = liftGraphQLCodes(err);
        if (codes.includes("UNAUTHENTICATED")) return { status: "auth", kind: "unauthenticated" };
        if (codes.includes("FORBIDDEN")) return { status: "auth", kind: "forbidden" };
        // err.message is omitted — backend messages may echo user input.
        // codes is safe to log (fixed enum of GraphQL extension codes).
        console.warn("[useCreateCardgroup] createCardgroup rejected", {
          name: err instanceof Error ? err.name : "unknown",
          codes,
        });
        return { status: "rejected" };
      }
    },
    [createCardgroup],
  );

  return { create, loading };
}
