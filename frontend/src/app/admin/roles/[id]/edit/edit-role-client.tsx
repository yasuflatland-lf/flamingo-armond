"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { RoleForm } from "@/components/admin/role-form";
import { AdminUpdateRoleMutation, isSystemRoleName } from "../../queries";

export type RoleForEdit = { id: string; name: string };

type Props = {
  role: RoleForEdit;
};

export function EditRoleClient({ role }: Props) {
  const router = useRouter();
  const readOnly = isSystemRoleName(role.name);

  const [updateRole, { loading, error }] = useMutation(AdminUpdateRoleMutation, {
    onCompleted(data) {
      if (!data?.updateRole) return;
      router.push("/admin/roles");
      router.refresh();
    },
  });

  async function handleSubmit(values: { name: string }) {
    await updateRole({ variables: { id: role.id, name: values.name } }).catch((err) => {
      // err.message is omitted — backend messages may echo user input.
      console.warn("[admin/roles/:id/edit] updateRole rejected", {
        roleId: role.id,
        name: err instanceof Error ? err.name : "unknown",
      });
    });
  }

  return (
    <main className="p-8">
      <div className="mb-6 flex items-center gap-4">
        <Link href="/admin/roles" className="text-sm text-muted-foreground hover:underline">
          &larr; Back
        </Link>
        <h1 className="text-2xl font-semibold">Edit role</h1>
      </div>

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
        mode="edit"
        defaultValues={{ name: role.name }}
        submit={handleSubmit}
        submitting={loading}
        error={error}
        readOnly={readOnly}
      />
    </main>
  );
}
