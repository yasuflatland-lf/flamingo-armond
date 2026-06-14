"use client";

import { useApolloClient, useMutation } from "@apollo/client/react";
import type { Reference } from "@apollo/client/utilities";
import { useCallback } from "react";
import type { AdminMastersQuery as AdminMastersQueryResult } from "@/generated/graphql";
import { classifyMutationAuthError } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import type { MasterFormValues } from "./admin-master-form";
import {
  ADMIN_MASTERS_BASE_VARS,
  AdminCreateMasterMutation,
  AdminDeleteMasterMutation,
  AdminMastersQuery,
  AdminPublishMasterMutation,
  AdminUnpublishMasterMutation,
  AdminUpdateMasterMutation,
} from "./queries";

/** Auth-relevant failure kind surfaced to the caller for toast copy selection. */
export type AuthKind = "forbidden" | "unauthenticated";

export type CreateMasterOutcome =
  | { status: "success" }
  | { status: "validation"; field: string; message: string }
  | { status: "auth"; kind: AuthKind }
  | { status: "unexpected" }
  | { status: "rejected" };

export type UpdateMasterOutcome =
  | { status: "success" }
  | { status: "validation"; field: string; message: string }
  | { status: "auth"; kind: AuthKind }
  | { status: "unexpected" }
  | { status: "rejected" };

export type DeleteMasterOutcome =
  | { status: "success" }
  | { status: "auth"; kind: AuthKind }
  | { status: "rejected" };

export type PublishMasterOutcome =
  | { status: "success" }
  | { status: "empty" }
  | { status: "auth"; kind: AuthKind }
  | { status: "unexpected" }
  | { status: "rejected" };

export type UnpublishMasterOutcome =
  | { status: "success" }
  | { status: "auth"; kind: AuthKind }
  | { status: "unexpected" }
  | { status: "rejected" };

// FORBIDDEN / UNAUTHENTICATED → an auth outcome; anything else → null so the
// caller falls through to its own rejected/unexpected branch.
function authOutcome(err: unknown): { status: "auth"; kind: AuthKind } | null {
  const kind = classifyMutationAuthError(err);
  if (kind === "other") return null;
  return { status: "auth", kind };
}

/**
 * Owns the five admin-masters mutations and their Apollo cache writes. The client
 * maps the returned typed outcomes to toast copy, sheet navigation, and validation
 * state. Mirrors the thin-outcome-hook shape of `useCreateCardgroup` /
 * `useImportMaster`.
 *
 * No `optimisticResponse`: typed errors (InputValidationError, FORBIDDEN) can fail
 * these mutations and Apollo does not reliably roll back optimistic writes for
 * typed GraphQL errors — see .claude/rules/pagination.md.
 */
export function useMasterMutations() {
  const apolloClient = useApolloClient();

  const [runCreate, { loading: creating, reset: resetCreate }] = useMutation(
    AdminCreateMasterMutation,
    {
      update(cache, { data }) {
        // Narrow before touching .master so a validation/unknown variant cannot
        // mutate the cache.
        if (data?.adminCreateMasterCardgroup?.__typename !== "CreateMasterCardgroupSuccess") return;
        const created = data.adminCreateMasterCardgroup.master;
        // cache.modify is forbidden — readQuery + writeQuery handles the cold cache
        // too. Write the search=null variant only; ADMIN_MASTERS_BASE_VARS keeps the
        // key in sync with the client useConnectionPagination query. See
        // .claude/rules/pagination.md.
        const existing = cache.readQuery<AdminMastersQueryResult>({
          query: AdminMastersQuery,
          variables: { ...ADMIN_MASTERS_BASE_VARS, search: null },
        });
        const createdEdge = {
          __typename: "MasterCatalogEdge" as const,
          cursor: created.id,
          node: created,
        };
        cache.writeQuery<AdminMastersQueryResult>({
          query: AdminMastersQuery,
          variables: { ...ADMIN_MASTERS_BASE_VARS, search: null },
          data: existing?.adminMasters
            ? {
                adminMasters: {
                  ...existing.adminMasters,
                  edges: [createdEdge, ...existing.adminMasters.edges],
                  totalCount: existing.adminMasters.totalCount + 1,
                },
              }
            : {
                adminMasters: {
                  __typename: "MasterCatalogConnection",
                  edges: [createdEdge],
                  pageInfo: {
                    __typename: "PageInfo",
                    hasNextPage: false,
                    hasPreviousPage: false,
                    startCursor: created.id,
                    endCursor: created.id,
                  },
                  totalCount: 1,
                },
              },
        });
      },
    },
  );
  const [runUpdate, { loading: updating, reset: resetUpdate }] =
    useMutation(AdminUpdateMasterMutation);
  const [runDelete] = useMutation(AdminDeleteMasterMutation);
  const [runPublish] = useMutation(AdminPublishMasterMutation);
  const [runUnpublish] = useMutation(AdminUnpublishMasterMutation);

  const createMaster = useCallback(
    async (values: MasterFormValues): Promise<CreateMasterOutcome> => {
      try {
        const result = await runCreate({ variables: { input: values } });
        const payload = result.data?.adminCreateMasterCardgroup;
        const typename = payload?.__typename ?? null;
        if (payload?.__typename === "InputValidationError") {
          return { status: "validation", field: payload.field, message: payload.message };
        }
        if (payload?.__typename === "CreateMasterCardgroupSuccess") {
          return { status: "success" };
        }
        console.warn("[useMasterMutations] unexpected createMaster payload", { typename });
        return { status: "unexpected" };
      } catch (err) {
        const auth = authOutcome(err);
        if (auth) return auth;
        console.warn("[useMasterMutations] createMaster rejected", {
          name: err instanceof Error ? err.name : "unknown",
          codes: liftGraphQLCodes(err),
        });
        return { status: "rejected" };
      }
    },
    [runCreate],
  );

  const updateMaster = useCallback(
    async (id: string, values: MasterFormValues): Promise<UpdateMasterOutcome> => {
      try {
        const result = await runUpdate({ variables: { id, input: values } });
        const payload = result.data?.adminUpdateMasterCardgroup;
        const typename = payload?.__typename ?? null;
        if (payload?.__typename === "InputValidationError") {
          return { status: "validation", field: payload.field, message: payload.message };
        }
        if (payload?.__typename === "UpdateMasterCardgroupSuccess") {
          return { status: "success" };
        }
        console.warn("[useMasterMutations] unexpected updateMaster payload", { typename });
        return { status: "unexpected" };
      } catch (err) {
        const auth = authOutcome(err);
        if (auth) return auth;
        console.warn("[useMasterMutations] updateMaster rejected", {
          masterId: id,
          name: err instanceof Error ? err.name : "unknown",
          codes: liftGraphQLCodes(err),
        });
        return { status: "rejected" };
      }
    },
    [runUpdate],
  );

  const deleteMaster = useCallback(
    async (id: string): Promise<DeleteMasterOutcome> => {
      try {
        const result = await runDelete({ variables: { id } });
        if (!result.data?.adminDeleteMasterCardgroup) {
          throw new Error("adminDeleteMasterCardgroup returned false");
        }
        apolloClient.cache.modify({
          fields: {
            adminMasters(existing, { readField }) {
              const conn = existing as {
                edges?: ReadonlyArray<{ node: Reference }>;
                totalCount?: number;
              };
              if (!conn.edges) return existing;
              // Filter by the normalized node id, not by `cursor` (the edge cursor is
              // an opaque "v1:..." value that never equals the raw id).
              const next = conn.edges.filter((edge) => readField<string>("id", edge.node) !== id);
              if (next.length === conn.edges.length) return existing;
              return { ...conn, edges: next, totalCount: Math.max(0, (conn.totalCount ?? 0) - 1) };
            },
          },
        });
        const cacheId = apolloClient.cache.identify({ __typename: "MasterCardgroup", id });
        if (cacheId) {
          apolloClient.cache.evict({ id: cacheId });
          apolloClient.cache.gc();
        }
        return { status: "success" };
      } catch (err) {
        const auth = authOutcome(err);
        if (auth) return auth;
        console.warn("[useMasterMutations] deleteMaster rejected", {
          masterId: id,
          name: err instanceof Error ? err.name : "unknown",
          codes: liftGraphQLCodes(err),
        });
        return { status: "rejected" };
      }
    },
    [apolloClient, runDelete],
  );

  const publishMaster = useCallback(
    async (id: string): Promise<PublishMasterOutcome> => {
      try {
        const result = await runPublish({ variables: { id } });
        const payload = result.data?.adminPublishMasterCardgroup;
        const typename = payload?.__typename ?? null;
        if (payload?.__typename === "PublishMasterCardgroupSuccess") {
          return { status: "success" };
        }
        if (payload?.__typename === "MasterCardgroupEmptyError") {
          return { status: "empty" };
        }
        console.warn("[useMasterMutations] unexpected publishMaster payload", {
          masterId: id,
          typename,
        });
        return { status: "unexpected" };
      } catch (err) {
        const auth = authOutcome(err);
        if (auth) return auth;
        console.warn("[useMasterMutations] publishMaster rejected", {
          masterId: id,
          name: err instanceof Error ? err.name : "unknown",
          codes: liftGraphQLCodes(err),
        });
        return { status: "rejected" };
      }
    },
    [runPublish],
  );

  const unpublishMaster = useCallback(
    async (id: string): Promise<UnpublishMasterOutcome> => {
      try {
        const result = await runUnpublish({ variables: { id } });
        if (!result.data?.adminUnpublishMasterCardgroup) {
          console.warn("[useMasterMutations] unpublishMaster returned null payload", {
            masterId: id,
          });
          return { status: "unexpected" };
        }
        return { status: "success" };
      } catch (err) {
        const auth = authOutcome(err);
        if (auth) return auth;
        console.warn("[useMasterMutations] unpublishMaster rejected", {
          masterId: id,
          name: err instanceof Error ? err.name : "unknown",
          codes: liftGraphQLCodes(err),
        });
        return { status: "rejected" };
      }
    },
    [runUnpublish],
  );

  return {
    createMaster,
    updateMaster,
    deleteMaster,
    publishMaster,
    unpublishMaster,
    creating,
    updating,
    resetCreate,
    resetUpdate,
  };
}
