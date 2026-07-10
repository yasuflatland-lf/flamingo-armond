"use client";

import { useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { classifyAndLogAuthOutcome } from "@/lib/apollo/errors";
import { prependMyCardgroupEdge } from "./cache";
import { CreateCardgroupMutation } from "./queries";

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
  | { status: "limit"; limit: number; current: number }
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
      prependMyCardgroupEdge(cache, data.createCardgroup.cardgroup);
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
        if (payload?.__typename === "CardgroupLimitReachedError") {
          return { status: "limit", limit: payload.limit, current: payload.current };
        }
        if (payload?.__typename === "CreateCardgroupSuccess") {
          return { status: "success", cardgroupId: payload.cardgroup.id };
        }
        // Null payload, partial-response null bubble, or a future union variant
        // the client was not regenerated against.
        console.warn("[useCreateCardgroup] unexpected createCardgroup payload", { typename });
        return { status: "unexpected" };
      } catch (err) {
        // FORBIDDEN / UNAUTHENTICATED → typed auth outcome; otherwise a scoped
        // structured warn (omitting err.message, which may echo user input) and
        // a rejected outcome. See lib/apollo/errors classifyAndLogAuthOutcome.
        return classifyAndLogAuthOutcome(err, "useCreateCardgroup", "createCardgroup");
      }
    },
    [createCardgroup],
  );

  return { create, loading };
}
