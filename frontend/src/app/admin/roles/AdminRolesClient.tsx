"use client";

import { useMutation } from "@apollo/client/react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getBackendErrorBanner, getBackendFieldErrors } from "@/lib/apollo/errors";
import {
  AdminCreateRoleMutation,
  AdminDeleteRoleMutation,
  AdminUpdateRoleMutation,
} from "./queries";

export type RoleItem = { id: string; name: string };

type Props = { initialRoles: RoleItem[] };

/** Convert any thrown value to a user-facing string. */
function toMessage(err: unknown): string {
  return getBackendErrorBanner(err) ?? (err instanceof Error ? err.message : String(err));
}

/**
 * Client component for the /admin/roles page.
 * Supports listing, inline-edit, add, and delete of roles.
 * The "admin" system role has Edit/Delete disabled (server also enforces this via FORBIDDEN).
 * No optimisticResponse is used — typed-error mutations can fail with FORBIDDEN / BAD_USER_INPUT
 * and the cache must stay truthful (see .claude/rules/pagination.md).
 */
export function AdminRolesClient({ initialRoles }: Props) {
  const [roles, setRoles] = useState<RoleItem[]>(initialRoles);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [newName, setNewName] = useState("");

  // Banner error for operations that are not field-level (delete, FORBIDDEN, network, etc.)
  const [error, setError] = useState<string | null>(null);

  // Field-level errors are kept separate per context to avoid cross-row leakage.
  const [addRowFieldErrors, setAddRowFieldErrors] = useState<Record<string, string>>({});
  const [editRowFieldErrors, setEditRowFieldErrors] = useState<Record<string, string>>({});

  const [createRoleMutate, { loading: creating }] = useMutation(AdminCreateRoleMutation);
  const [updateRoleMutate, { loading: updating }] = useMutation(AdminUpdateRoleMutation);
  const [deleteRoleMutate, { loading: deleting }] = useMutation(AdminDeleteRoleMutation);

  const busy = creating || updating || deleting;

  /** The "admin" system role must not be edited or deleted from the UI. */
  const isSystemRole = (role: RoleItem) => role.name === "admin";

  function handleStartEdit(role: RoleItem) {
    setEditingId(role.id);
    setDraft(role.name);
    setError(null);
    setEditRowFieldErrors({});
  }

  function handleCancelEdit() {
    setEditingId(null);
    setDraft("");
    setEditRowFieldErrors({});
  }

  async function handleSaveEdit(id: string) {
    setError(null);
    setEditRowFieldErrors({});
    const trimmed = draft.trim();
    if (!trimmed) {
      setError("Name is required.");
      return;
    }
    try {
      const res = await updateRoleMutate({ variables: { id, name: trimmed } });
      // Fragment masking is compile-time only; runtime shape is the plain object.
      const updated = res.data?.updateRole as RoleItem | undefined;
      if (updated) {
        setRoles((prev) => prev.map((r) => (r.id === id ? updated : r)));
        setEditingId(null);
        setDraft("");
      }
    } catch (err) {
      const banner = getBackendErrorBanner(err);
      const fields = getBackendFieldErrors(err);
      if (Object.keys(fields).length > 0) setEditRowFieldErrors(fields);
      if (banner) setError(banner);
      if (!banner && Object.keys(fields).length === 0) setError(toMessage(err));
    }
  }

  async function handleCreate() {
    setError(null);
    setAddRowFieldErrors({});
    const trimmed = newName.trim();
    if (!trimmed) {
      setError("Name is required.");
      return;
    }
    try {
      const res = await createRoleMutate({ variables: { name: trimmed } });
      const created = res.data?.createRole as RoleItem | undefined;
      if (created) {
        setRoles((prev) => [...prev, created]);
        setNewName("");
      }
    } catch (err) {
      const banner = getBackendErrorBanner(err);
      const fields = getBackendFieldErrors(err);
      if (Object.keys(fields).length > 0) setAddRowFieldErrors(fields);
      if (banner) setError(banner);
      if (!banner && Object.keys(fields).length === 0) setError(toMessage(err));
    }
  }

  async function handleDelete(id: string) {
    setError(null);
    try {
      await deleteRoleMutate({ variables: { id } });
      setRoles((prev) => prev.filter((r) => r.id !== id));
    } catch (err) {
      setError(toMessage(err));
    }
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4 p-8">
      <h1 className="text-2xl font-semibold">Roles</h1>

      {error && (
        <div
          role="alert"
          data-testid="admin-roles-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {error}
        </div>
      )}

      <ul
        className="divide-y divide-border rounded-md border border-border"
        data-testid="admin-roles-list"
      >
        {roles.map((role) => (
          <li
            key={role.id}
            className="flex flex-col gap-1 p-3"
            data-testid={`admin-role-row-${role.id}`}
          >
            {editingId === role.id ? (
              <div className="flex items-start gap-3">
                <div className="flex flex-col gap-1">
                  <Input
                    id={`edit-name-${role.id}`}
                    value={draft}
                    onChange={(e) => setDraft(e.target.value)}
                    className="max-w-xs"
                    aria-label="Role name"
                    aria-invalid={editRowFieldErrors.name ? "true" : undefined}
                    aria-describedby={
                      editRowFieldErrors.name ? `edit-name-error-${role.id}` : undefined
                    }
                    data-testid="admin-role-edit-input"
                  />
                  {editRowFieldErrors.name && (
                    <p
                      id={`edit-name-error-${role.id}`}
                      role="alert"
                      data-testid="admin-role-edit-field-error"
                      className="text-xs text-destructive mt-1"
                    >
                      {editRowFieldErrors.name}
                    </p>
                  )}
                </div>
                <Button
                  type="button"
                  onClick={() => handleSaveEdit(role.id)}
                  disabled={busy}
                  data-testid="admin-role-save-btn"
                >
                  Save
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={handleCancelEdit}
                  disabled={busy}
                  data-testid="admin-role-cancel-btn"
                >
                  Cancel
                </Button>
              </div>
            ) : (
              <div className="flex items-center gap-3">
                <span className="flex-1" data-testid="admin-role-name">
                  {role.name}
                </span>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => handleStartEdit(role)}
                  disabled={busy || isSystemRole(role)}
                  aria-label={`Edit ${role.name}`}
                  data-testid={`admin-role-edit-btn-${role.id}`}
                >
                  Edit
                </Button>
                <Button
                  type="button"
                  variant="destructive"
                  size="sm"
                  onClick={() => handleDelete(role.id)}
                  disabled={busy || isSystemRole(role)}
                  aria-label={`Delete ${role.name}`}
                  data-testid={`admin-role-delete-btn-${role.id}`}
                >
                  Delete
                </Button>
              </div>
            )}
          </li>
        ))}

        {/* Add-role row */}
        <li className="flex flex-col gap-1 bg-muted/30 p-3">
          <div className="flex items-start gap-3">
            <div className="flex flex-col gap-1">
              <Input
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="new-role-name"
                className="max-w-xs"
                aria-label="New role name"
                aria-invalid={addRowFieldErrors.name ? "true" : undefined}
                aria-describedby={addRowFieldErrors.name ? "add-role-name-error" : undefined}
                data-testid="admin-role-new-name-input"
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleCreate();
                }}
              />
              {addRowFieldErrors.name && (
                <p
                  id="add-role-name-error"
                  role="alert"
                  data-testid="admin-role-add-field-error"
                  className="text-xs text-destructive mt-1"
                >
                  {addRowFieldErrors.name}
                </p>
              )}
            </div>
            <Button
              type="button"
              onClick={handleCreate}
              disabled={busy}
              data-testid="admin-role-add-btn"
            >
              Add role
            </Button>
          </div>
        </li>
      </ul>
    </div>
  );
}
