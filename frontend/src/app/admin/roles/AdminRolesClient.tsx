"use client";

import { useMutation } from "@apollo/client/react";
import { Plus } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { RoleListItem } from "@/components/admin/role-list-item";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Button } from "@/components/ui/button";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { AdminDeleteRoleMutation, SYSTEM_ROLE_NAMES } from "./queries";

export type RoleItem = { id: string; name: string };

type Props = { initialRoles: RoleItem[] };

/** Convert any thrown value to a user-facing string. */
function toMessage(err: unknown): string {
  return getBackendErrorBanner(err) ?? (err instanceof Error ? err.message : String(err));
}

/**
 * Roles listing page. Add and edit are dedicated routes
 * (/admin/roles/new and /admin/roles/:id/edit) so this component is a thin
 * presentational shell over the server-fetched role list. The "admin" and
 * "general" system roles render as non-clickable rows with delete disabled;
 * the backend also enforces the guard via FORBIDDEN.
 *
 * No optimisticResponse on delete: a typed FORBIDDEN can fail the mutation,
 * and Apollo does not consistently roll back optimistic writes for typed
 * GraphQL errors (see docs/pagination/drop-optimistic-response-typed-errors.md).
 */
export function AdminRolesClient({ initialRoles }: Props) {
  const [roles, setRoles] = useState<RoleItem[]>(initialRoles);
  const [error, setError] = useState<string | null>(null);

  const [deleteRoleMutate, { loading: deleting }] = useMutation(AdminDeleteRoleMutation);

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
    <ListingPageShell
      title="Roles"
      description="Manage roles available to assign to users."
      primaryActions={
        <Button asChild variant="brand" data-testid="admin-roles-new-btn">
          <Link href="/admin/roles/new">
            <span>New role</span>
            <Plus aria-hidden="true" />
          </Link>
        </Button>
      }
    >
      {error && (
        <div
          role="alert"
          data-testid="admin-roles-error"
          className="rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {error}
        </div>
      )}

      <ul className="space-y-3" data-testid="admin-roles-list">
        {roles.map((role) => (
          <RoleListItem
            key={role.id}
            id={role.id}
            name={role.name}
            isSystem={SYSTEM_ROLE_NAMES.has(role.name)}
            busy={deleting}
            onDelete={handleDelete}
          />
        ))}
      </ul>
    </ListingPageShell>
  );
}
