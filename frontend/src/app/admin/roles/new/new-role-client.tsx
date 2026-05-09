"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { RoleForm } from "@/components/admin/role-form";
import { AdminCreateRoleMutation } from "../queries";

export function NewRoleClient() {
  const router = useRouter();

  // No optimisticResponse: typed errors (FORBIDDEN, BAD_USER_INPUT for
  // duplicate name) can fail the mutation, and Apollo does not consistently
  // roll back optimistic writes for typed GraphQL errors. The user pays one
  // round-trip of latency in exchange for a truthful cache.
  const [createRole, { loading, error }] = useMutation(AdminCreateRoleMutation, {
    onCompleted(data) {
      if (!data?.createRole) return;
      router.push("/admin/roles");
      router.refresh();
    },
  });

  async function handleSubmit(values: { name: string }) {
    await createRole({ variables: { name: values.name } }).catch((err) => {
      // err.message is omitted — backend messages may echo user input.
      console.warn("[admin/roles/new] createRole rejected", {
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
        <h1 className="text-2xl font-semibold">New role</h1>
      </div>
      <RoleForm
        mode="create"
        defaultValues={{ name: "" }}
        submit={handleSubmit}
        submitting={loading}
        error={error}
      />
    </main>
  );
}
