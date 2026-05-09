"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
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

  const [updateRole, { loading, error }] = useMutation(AdminUpdateRoleMutation, {
    onCompleted(data) {
      if (!data?.updateRole) return;
      router.push("/admin/roles");
      router.refresh();
    },
  });

  async function handleSubmit(values: { name: string }) {
    // Mirror the backend `validateRoleName` normalization — see new-role-client.tsx.
    const name = values.name.trim().toLowerCase();
    await updateRole({ variables: { id: role.id, name } }).catch((err) => {
      // err.message is omitted — backend messages may echo user input.
      console.warn("[admin/roles/:id/edit] updateRole rejected", {
        roleId: role.id,
        name: err instanceof Error ? err.name : "unknown",
      });
    });
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
