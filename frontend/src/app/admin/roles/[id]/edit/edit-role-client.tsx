"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { RoleForm } from "@/components/admin/role-form";
import { Button } from "@/components/ui/button";
import { AdminUpdateRoleMutation, SYSTEM_ROLE_NAMES } from "../../queries";

export type RoleForEdit = { id: string; name: string };

type Props = {
  role: RoleForEdit;
};

export function EditRoleClient({ role }: Props) {
  const router = useRouter();
  const readOnly = SYSTEM_ROLE_NAMES.has(role.name);

  // Typed error from the CannotModifySystemRoleError union variant.
  // Cleared on each new submission attempt.
  const [systemRoleError, setSystemRoleError] = useState<string | null>(null);

  // No optimisticResponse: CannotModifySystemRoleError is a typed union variant
  // and Apollo v3.x does not roll back optimistic writes on typed GraphQL errors
  // — see docs/pagination/drop-optimistic-response-typed-errors.md.
  const [updateRole, { loading, error }] = useMutation(AdminUpdateRoleMutation);

  async function handleSubmit(values: { name: string }) {
    setSystemRoleError(null);
    // Mirror the backend `validateRoleName` normalization — see new-role-client.tsx.
    const name = values.name.trim().toLowerCase();
    const result = await updateRole({ variables: { id: role.id, name } }).catch((err) => {
      // err.message is omitted — backend messages may echo user input.
      console.warn("[admin/roles/:id/edit] updateRole rejected", {
        roleId: role.id,
        name: err instanceof Error ? err.name : "unknown",
      });
      return null;
    });
    if (!result) return;
    const payload = result.data?.updateRole;
    if (payload?.__typename === "CannotModifySystemRoleError") {
      // Domain invariant: system roles are immutable. Surface as a banner;
      // do not navigate and do not mutate the Apollo cache.
      setSystemRoleError(payload.message);
      return;
    }
    // payload.__typename === "UpdateRoleSuccess"
    if (payload?.__typename === "UpdateRoleSuccess") {
      router.push("/admin/roles");
      router.refresh();
    }
  }

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">Edit role</h1>

      {readOnly ? (
        <div
          role="status"
          data-testid="admin-role-edit-system-banner"
          className="mb-4 rounded-md border border-border bg-muted/30 p-3 text-sm text-muted-foreground"
        >
          This is a system role. Its name cannot be changed.
        </div>
      ) : null}

      {systemRoleError ? (
        <div
          role="alert"
          data-testid="admin-role-edit-system-role-error"
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          {systemRoleError}
        </div>
      ) : null}

      <RoleForm
        defaultValues={{ name: role.name }}
        submit={handleSubmit}
        submitLabel="Save"
        submitting={loading}
        error={error}
        readOnly={readOnly}
        secondarySlot={
          <Button asChild variant="outline">
            <Link href="/admin/roles">Cancel</Link>
          </Button>
        }
      />
    </main>
  );
}
