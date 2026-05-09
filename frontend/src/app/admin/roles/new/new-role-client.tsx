"use client";

import { useMutation } from "@apollo/client/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { RoleForm } from "@/components/admin/role-form";
import { Button } from "@/components/ui/button";
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
    // Mirror the backend `validateRoleName` normalization so the mutation
    // always sees the canonical form. TanStack Form's `value` is the raw
    // user input — Zod's `transform` runs in validators only, not on submit.
    const name = values.name.trim().toLowerCase();
    await createRole({ variables: { name } }).catch((err) => {
      // err.message is omitted — backend messages may echo user input.
      console.warn("[admin/roles/new] createRole rejected", {
        name: err instanceof Error ? err.name : "unknown",
      });
    });
  }

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">New role</h1>
      <RoleForm
        defaultValues={{ name: "" }}
        submit={handleSubmit}
        submitLabel="Create"
        submitting={loading}
        error={error}
        secondarySlot={
          <Button asChild variant="outline">
            <Link href="/admin/roles">Cancel</Link>
          </Button>
        }
      />
    </main>
  );
}
