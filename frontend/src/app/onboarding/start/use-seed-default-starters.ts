"use client";

import { useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { prependMyCardgroupEdge } from "@/app/cardgroups/cache";
import { classifyMutationAuthError } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { SeedDefaultStartersMutation } from "./queries";

export type SeedDefaultStartersOutcome =
  | { status: "success"; count: number }
  | { status: "auth"; kind: "unauthenticated" | "forbidden" }
  | { status: "rejected" };

/**
 * Wraps `seedDefaultStarterCardgroups` and, on success, prepends every seeded
 * cardgroup to the `myCardgroupsConnection` cache so they appear on `/cardgroups`
 * without a refetch. No `optimisticResponse`: the mutation can fail with
 * UNAUTHENTICATED and Apollo does not consistently roll back optimistic writes
 * for typed GraphQL errors (see .claude/rules/pagination.md).
 */
export function useSeedDefaultStarters() {
  const [seed, { loading }] = useMutation(SeedDefaultStartersMutation, {
    update(cache, { data }) {
      const cgs = data?.seedDefaultStarterCardgroups?.cardgroups;
      if (!cgs) return;
      for (const cg of cgs) prependMyCardgroupEdge(cache, cg);
    },
  });

  const seedDefaultStarters = useCallback(async (): Promise<SeedDefaultStartersOutcome> => {
    try {
      const result = await seed();
      const cgs = result.data?.seedDefaultStarterCardgroups?.cardgroups;
      if (!cgs) {
        console.warn("[useSeedDefaultStarters] unexpected payload");
        return { status: "rejected" };
      }
      return { status: "success", count: cgs.length };
    } catch (err) {
      const authKind = classifyMutationAuthError(err);
      if (authKind !== "other") return { status: "auth", kind: authKind };
      console.warn("[useSeedDefaultStarters] rejected", {
        name: err instanceof Error ? err.name : "unknown",
        codes: liftGraphQLCodes(err),
      });
      return { status: "rejected" };
    }
  }, [seed]);

  return { seedDefaultStarters, loading };
}
