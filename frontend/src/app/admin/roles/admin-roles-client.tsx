"use client";

import { useLazyQuery, useMutation } from "@apollo/client/react";
import { Plus } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { RoleForm } from "@/components/admin/role-form";
import { RoleListItem } from "@/components/admin/role-list-item";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Button } from "@/components/ui/button";
import { FormSheet, useFormSheetClose } from "@/components/ui/form-sheet";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";
import { useUndoDelete } from "@/lib/undo-delete";
import { useSheetSearchParam } from "@/lib/url/use-sheet-search-param";
import {
  AdminCreateRoleMutation,
  AdminDeleteRoleMutation,
  AdminRoleQuery,
  AdminUpdateRoleMutation,
  SYSTEM_ROLE_NAMES,
} from "./queries";

export type RoleItem = { id: string; name: string };

type Props = { initialRoles: RoleItem[] };

type ValidationError = { field: string; message: string };

function AuthBanner({
  testId,
  authError,
}: {
  testId: string;
  authError: "unauthenticated" | "forbidden";
}) {
  return (
    <div
      role="alert"
      data-testid={testId}
      className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
    >
      <span>
        {authError === "unauthenticated"
          ? "Your session has expired. "
          : "You do not have permission. "}
      </span>
      <Link href="/login" className="underline">
        Sign in again
      </Link>
      .
    </div>
  );
}

function CreateRoleSheetBody({
  submitting,
  validationError,
  authError,
  unexpectedPayloadError,
  onDirty,
  onDirtyChange,
  submit,
}: {
  submitting: boolean;
  validationError: ValidationError | null;
  authError: "unauthenticated" | "forbidden" | null;
  unexpectedPayloadError: string | null;
  onDirty: () => void;
  onDirtyChange: (dirty: boolean) => void;
  submit: (values: { name: string }) => Promise<void>;
}) {
  const close = useFormSheetClose();

  return (
    <div onInput={onDirty}>
      {authError ? <AuthBanner testId="admin-role-new-auth-error" authError={authError} /> : null}
      {validationError ? (
        <div
          role="alert"
          data-testid="admin-role-new-validation-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {validationError.message}
        </div>
      ) : null}
      {unexpectedPayloadError ? (
        <div
          role="alert"
          data-testid="admin-role-new-unexpected-payload-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {unexpectedPayloadError}
        </div>
      ) : null}

      <RoleForm
        defaultValues={{ name: "" }}
        submit={submit}
        submitLabel="Create"
        submitting={submitting}
        onCancel={close}
        onDirtyChange={onDirtyChange}
      />
    </div>
  );
}

function EditRoleSheetBody({
  role,
  submitting,
  mutationError,
  authError,
  systemRoleError,
  unexpectedPayloadError,
  queryErrorBanner,
  submit,
}: {
  role: RoleItem | null;
  submitting: boolean;
  mutationError: unknown;
  authError: "unauthenticated" | "forbidden" | null;
  systemRoleError: string | null;
  unexpectedPayloadError: string | null;
  queryErrorBanner: string | undefined;
  submit: (values: { name: string }) => Promise<void>;
}) {
  const close = useFormSheetClose();

  if (!role) {
    return (
      <div className="space-y-4">
        {queryErrorBanner ? (
          <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {queryErrorBanner}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">Loading role...</p>
        )}
      </div>
    );
  }

  const readOnly = SYSTEM_ROLE_NAMES.has(role.name);

  return (
    <div className="space-y-4">
      {queryErrorBanner ? (
        <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {queryErrorBanner}
        </div>
      ) : null}
      {readOnly ? (
        <div
          role="status"
          data-testid="admin-role-edit-system-banner"
          className="rounded-md border border-border bg-muted/30 p-3 text-sm text-muted-foreground"
        >
          This is a system role. Its name cannot be changed.
        </div>
      ) : null}
      {authError ? <AuthBanner testId="admin-role-edit-auth-error" authError={authError} /> : null}
      {systemRoleError ? (
        <div
          role="alert"
          data-testid="admin-role-edit-system-role-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {systemRoleError}
        </div>
      ) : null}
      {unexpectedPayloadError ? (
        <div
          role="alert"
          data-testid="admin-role-edit-unexpected-payload-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {unexpectedPayloadError}
        </div>
      ) : null}

      <RoleForm
        defaultValues={{ name: role.name }}
        submit={submit}
        submitLabel="Save"
        submitting={submitting}
        error={mutationError}
        readOnly={readOnly}
        onCancel={close}
      />
    </div>
  );
}

/** Convert any thrown value to a user-facing string. */
function toMessage(err: unknown): string {
  return getBackendErrorBanner(err) ?? (err instanceof Error ? err.message : String(err));
}

export function AdminRolesClient({ initialRoles }: Props) {
  const [roles, setRoles] = useState<RoleItem[]>(initialRoles);
  const [createDirty, setCreateDirty] = useState(false);
  const [createValidationError, setCreateValidationError] = useState<ValidationError | null>(null);
  const [createUnexpectedPayloadError, setCreateUnexpectedPayloadError] = useState<string | null>(
    null,
  );
  const [createAuthError, setCreateAuthError] = useState<"unauthenticated" | "forbidden" | null>(
    null,
  );
  const [editSystemRoleError, setEditSystemRoleError] = useState<string | null>(null);
  const [editUnexpectedPayloadError, setEditUnexpectedPayloadError] = useState<string | null>(null);
  const [editAuthError, setEditAuthError] = useState<"unauthenticated" | "forbidden" | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const [createRole, { loading: creating, reset: resetCreateRole }] =
    useMutation(AdminCreateRoleMutation);
  const [updateRole, { loading: updating, error: updateMutationError, reset: resetUpdateRole }] =
    useMutation(AdminUpdateRoleMutation);
  const [deleteRoleMutate, { loading: deleting }] = useMutation(AdminDeleteRoleMutation);
  const [loadRole, { data: editRoleData, loading: loadingEditRole, error: editRoleQueryError }] =
    useLazyQuery(AdminRoleQuery, { fetchPolicy: "no-cache" });
  const { scheduleDelete } = useUndoDelete();
  const { state, open, close } = useSheetSearchParam();
  const sheetMode = state.mode;
  const editId = state.mode === "edit" ? state.id : null;

  const resetCreateSheetState = useCallback(() => {
    setCreateValidationError(null);
    setCreateUnexpectedPayloadError(null);
    setCreateAuthError(null);
    setCreateDirty(false);
    resetCreateRole();
  }, [resetCreateRole]);

  const resetEditSheetState = useCallback(() => {
    setEditSystemRoleError(null);
    setEditUnexpectedPayloadError(null);
    setEditAuthError(null);
    resetUpdateRole();
  }, [resetUpdateRole]);

  useEffect(() => {
    setRoles(initialRoles);
  }, [initialRoles]);

  useEffect(() => {
    if (!editId) {
      return;
    }
    void loadRole({ variables: { id: editId } });
  }, [editId, loadRole]);

  useEffect(() => {
    switch (sheetMode) {
      case "new":
      case "closed":
      case "edit":
        resetCreateSheetState();
        break;
    }
  }, [resetCreateSheetState, sheetMode]);

  useEffect(() => {
    if (sheetMode === "edit" && editId === null) {
      return;
    }
    resetEditSheetState();
  }, [editId, resetEditSheetState, sheetMode]);

  const loadedEditRole = editRoleData?.role as unknown as RoleItem | undefined;
  const editRole = state.mode === "edit" && loadedEditRole?.id === state.id ? loadedEditRole : null;
  const editQueryErrorBanner = getBackendErrorBanner(editRoleQueryError);

  function handleDelete(id: string) {
    const index = roles.findIndex((r) => r.id === id);
    const role = roles[index];
    if (!role) return;

    // Clear any stale error banner so a new attempt starts clean.
    setDeleteError(null);
    setRoles((prev) => prev.filter((r) => r.id !== id));

    scheduleDelete({
      id,
      label: `Role "${role.name}" deleted`,
      optimisticRollback: () => {
        setRoles((prev) => [...prev.slice(0, index), role, ...prev.slice(index)]);
      },
      commitDelete: () => deleteRoleMutate({ variables: { id } }),
      onCommitFailed: (err) => setDeleteError(toMessage(err)),
    });
  }

  async function handleCreateSubmit(values: { name: string }) {
    setCreateValidationError(null);
    setCreateUnexpectedPayloadError(null);
    setCreateAuthError(null);

    const name = values.name.trim().toLowerCase();
    const result = await createRole({ variables: { name } }).catch((err) => {
      const codes = liftGraphQLCodes(err);
      if (codes.includes("UNAUTHENTICATED")) {
        setCreateAuthError("unauthenticated");
        return null;
      }
      if (codes.includes("FORBIDDEN")) {
        setCreateAuthError("forbidden");
        return null;
      }
      // err.message is omitted — backend messages may echo user input.
      // codes is safe to log (fixed enum of GraphQL extension codes).
      console.warn("[admin/roles] createRole rejected", {
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
      return null;
    });

    if (!result) return;

    const payload = result.data?.createRole;
    const typename = payload?.__typename ?? null;

    if (payload?.__typename === "InputValidationError") {
      setCreateValidationError({ field: payload.field, message: payload.message });
      return;
    }

    if (payload?.__typename === "CreateRoleSuccess") {
      resetCreateSheetState();
      close({ refresh: true });
      return;
    }

    console.warn("[admin/roles] unexpected createRole payload", {
      typename,
    });
    setCreateUnexpectedPayloadError("Something went wrong. Please try again.");
  }

  async function handleEditSubmit(values: { name: string }) {
    if (!editRole) return;

    setEditSystemRoleError(null);
    setEditUnexpectedPayloadError(null);
    setEditAuthError(null);

    const name = values.name.trim().toLowerCase();
    const result = await updateRole({ variables: { id: editRole.id, name } }).catch((err) => {
      const codes = liftGraphQLCodes(err);
      if (codes.includes("UNAUTHENTICATED")) {
        setEditAuthError("unauthenticated");
        return null;
      }
      if (codes.includes("FORBIDDEN")) {
        setEditAuthError("forbidden");
        return null;
      }
      console.warn("[admin/roles] updateRole rejected", {
        roleId: editRole.id,
        name: err instanceof Error ? err.name : "unknown",
        codes,
      });
      return null;
    });

    if (!result) return;

    const payload = result.data?.updateRole;
    const typename = payload?.__typename ?? null;

    if (payload?.__typename === "CannotModifySystemRoleError") {
      setEditSystemRoleError(payload.message);
      return;
    }

    if (payload?.__typename === "UpdateRoleSuccess") {
      resetEditSheetState();
      close({ refresh: true });
      return;
    }

    console.warn("[admin/roles] unexpected updateRole payload", {
      typename,
    });
    setEditUnexpectedPayloadError("Something went wrong. Please try again.");
  }

  return (
    <ListingPageShell
      title="Roles"
      description="Manage roles available to assign to users."
      primaryActions={
        <Button
          type="button"
          variant="brand"
          data-testid="admin-roles-new-btn"
          onClick={() => open({ mode: "new" })}
        >
          <span>New role</span>
          <Plus aria-hidden="true" />
        </Button>
      }
    >
      <ul className="space-y-3" data-testid="admin-roles-list">
        {deleteError ? (
          <li>
            <div
              role="alert"
              data-testid="admin-roles-error"
              className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
            >
              {deleteError}
            </div>
          </li>
        ) : null}
        {roles.map((role) => (
          <RoleListItem
            key={role.id}
            id={role.id}
            name={role.name}
            isSystem={SYSTEM_ROLE_NAMES.has(role.name)}
            busy={deleting}
            onEdit={(roleId) => open({ mode: "edit", id: roleId })}
            onDelete={handleDelete}
          />
        ))}
      </ul>

      <FormSheet
        title="New role"
        open={state.mode === "new"}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) {
            resetCreateSheetState();
            close();
          }
        }}
        submitting={creating}
        dirty={createDirty}
        confirmOnDismiss
        size="sm"
      >
        {state.mode === "new" ? (
          <CreateRoleSheetBody
            submitting={creating}
            validationError={createValidationError}
            authError={createAuthError}
            unexpectedPayloadError={createUnexpectedPayloadError}
            onDirty={() => {
              setCreateValidationError(null);
              setCreateUnexpectedPayloadError(null);
            }}
            onDirtyChange={setCreateDirty}
            submit={handleCreateSubmit}
          />
        ) : null}
      </FormSheet>

      <FormSheet
        title="Edit role"
        open={state.mode === "edit"}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) {
            resetEditSheetState();
            close();
          }
        }}
        submitting={updating || loadingEditRole}
        confirmOnDismiss={false}
        size="sm"
      >
        {state.mode === "edit" ? (
          <EditRoleSheetBody
            role={editRole}
            submitting={updating}
            mutationError={updateMutationError}
            authError={editAuthError}
            systemRoleError={editSystemRoleError}
            unexpectedPayloadError={editUnexpectedPayloadError}
            queryErrorBanner={editQueryErrorBanner}
            submit={handleEditSubmit}
          />
        ) : null}
      </FormSheet>
    </ListingPageShell>
  );
}
