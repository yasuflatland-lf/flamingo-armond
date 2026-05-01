"use client";

import { useMutation } from "@apollo/client/react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
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
  const [error, setError] = useState<string | null>(null);

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
  }

  function handleCancelEdit() {
    setEditingId(null);
    setDraft("");
  }

  async function handleSaveEdit(id: string) {
    setError(null);
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
      setError(toMessage(err));
    }
  }

  async function handleCreate() {
    setError(null);
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
      setError(toMessage(err));
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
            className="flex items-center gap-3 p-3"
            data-testid={`admin-role-row-${role.id}`}
          >
            {editingId === role.id ? (
              <>
                <Input
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  className="max-w-xs"
                  aria-label="Role name"
                  data-testid="admin-role-edit-input"
                />
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
              </>
            ) : (
              <>
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
              </>
            )}
          </li>
        ))}

        {/* Add-role row */}
        <li className="flex items-center gap-3 bg-muted/30 p-3">
          <Input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="new-role-name"
            className="max-w-xs"
            aria-label="New role name"
            data-testid="admin-role-new-name-input"
            onKeyDown={(e) => {
              if (e.key === "Enter") handleCreate();
            }}
          />
          <Button
            type="button"
            onClick={handleCreate}
            disabled={busy}
            data-testid="admin-role-add-btn"
          >
            Add role
          </Button>
        </li>
      </ul>
    </div>
  );
}
