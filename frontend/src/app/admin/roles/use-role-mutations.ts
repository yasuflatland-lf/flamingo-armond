"use client";

import { useLazyQuery, useMutation } from "@apollo/client/react";
import { useCallback } from "react";
import { classifyToAuthOutcome } from "@/lib/apollo/errors";
import {
  AdminCreateRoleMutation,
  AdminDeleteRoleMutation,
  AdminRoleQuery,
  AdminUpdateRoleMutation,
} from "./queries";

/** Auth-relevant failure kind surfaced to the caller for banner copy selection. */
export type AuthKind = "forbidden" | "unauthenticated";

export type CreateRoleOutcome =
  | { status: "success" }
  | { status: "validation"; field: string; message: string }
  | { status: "auth"; kind: AuthKind }
  | { status: "unexpected" }
  | { status: "rejected" };

export type UpdateRoleOutcome =
  | { status: "success" }
  | { status: "systemRole"; message: string }
  | { status: "auth"; kind: AuthKind }
  | { status: "unexpected" }
  | { status: "rejected" };

/** Normalize a role name the way the backend expects: trimmed, lower-cased. */
function normalizeName(name: string): string {
  return name.trim().toLowerCase();
}

/**
 * Owns the three admin-roles mutations (create / update / delete) plus the
 * edit-sheet lazy query. The client maps the returned typed outcomes to its own
 * banner state, sheet navigation, and the delete undo/optimistic orchestration.
 * Mirrors the thin-outcome-hook shape of `useMasterMutations`.
 *
 * `createRole` / `updateRole` narrow `payload.__typename` to a discriminated
 * outcome and fold any caught error through `classifyToAuthOutcome`. `deleteRole`
 * exposes only the raw mutation call: roles delete keeps its local `setRoles` +
 * `scheduleDelete` undo orchestration (and its FORBIDDEN-passthrough /
 * UNAUTHENTICATED-collapse handling) in the client, so the hook must return the
 * rejecting promise rather than a typed outcome.
 *
 * No `optimisticResponse`: typed errors (InputValidationError,
 * CannotModifySystemRoleError, FORBIDDEN) can fail these mutations and Apollo does
 * not reliably roll back optimistic writes for typed GraphQL errors — see
 * .claude/rules/pagination.md.
 */
export function useRoleMutations() {
  const [runCreate, { loading: creating, reset: resetCreateRole }] =
    useMutation(AdminCreateRoleMutation);
  const [runUpdate, { loading: updating, error: updateMutationError, reset: resetUpdateRole }] =
    useMutation(AdminUpdateRoleMutation);
  const [runDelete, { loading: deleting }] = useMutation(AdminDeleteRoleMutation);
  const [
    loadRole,
    {
      data: editRoleData,
      loading: loadingEditRole,
      error: editRoleQueryError,
      called: loadRoleCalled,
      variables: loadRoleVariables,
    },
  ] = useLazyQuery(AdminRoleQuery, { fetchPolicy: "no-cache" });

  const createRole = useCallback(
    async (values: { name: string }): Promise<CreateRoleOutcome> => {
      try {
        const result = await runCreate({ variables: { name: normalizeName(values.name) } });
        const payload = result.data?.createRole;
        const typename = payload?.__typename ?? null;
        if (payload?.__typename === "InputValidationError") {
          return { status: "validation", field: payload.field, message: payload.message };
        }
        if (payload?.__typename === "CreateRoleSuccess") {
          return { status: "success" };
        }
        console.warn("[useRoleMutations] unexpected createRole payload", { typename });
        return { status: "unexpected" };
      } catch (err) {
        return classifyToAuthOutcome<AuthKind>(err, "useRoleMutations", "createRole");
      }
    },
    [runCreate],
  );

  const updateRole = useCallback(
    async (id: string, values: { name: string }): Promise<UpdateRoleOutcome> => {
      try {
        const result = await runUpdate({ variables: { id, name: normalizeName(values.name) } });
        const payload = result.data?.updateRole;
        const typename = payload?.__typename ?? null;
        if (payload?.__typename === "CannotModifySystemRoleError") {
          return { status: "systemRole", message: payload.message };
        }
        if (payload?.__typename === "UpdateRoleSuccess") {
          return { status: "success" };
        }
        console.warn("[useRoleMutations] unexpected updateRole payload", { typename });
        return { status: "unexpected" };
      } catch (err) {
        return classifyToAuthOutcome<AuthKind>(err, "useRoleMutations", "updateRole", {
          roleId: id,
        });
      }
    },
    [runUpdate],
  );

  // Raw mutation call only. The client drives the optimistic delete + undo
  // toast via `scheduleDelete`; its `onCommitFailed` classifies the rejection
  // (FORBIDDEN message passthrough, UNAUTHENTICATED collapse), so the hook must
  // surface the rejecting promise rather than swallow it into a typed outcome.
  const deleteRole = useCallback((id: string) => runDelete({ variables: { id } }), [runDelete]);

  return {
    createRole,
    updateRole,
    deleteRole,
    loadRole,
    editRoleData,
    loadingEditRole,
    editRoleQueryError,
    loadRoleCalled,
    loadRoleVariables,
    updateMutationError,
    creating,
    updating,
    deleting,
    resetCreateRole,
    resetUpdateRole,
  };
}
