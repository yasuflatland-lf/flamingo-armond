import { redirect } from "next/navigation";
import { AdminRoleQuery } from "@/app/admin/roles/queries";
import type { AdminRoleQuery as AdminRoleQueryType } from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { EditRoleClient, type RoleForEdit } from "./edit-role-client";

type Props = {
  params: Promise<{ id: string }>;
};

/**
 * RSC for editing an existing role.
 *
 * admin/layout.tsx is the source-of-truth admin gate; the per-page getUser
 * here is defense-in-depth.
 *
 * UNAUTHENTICATED / FORBIDDEN from the role fetch redirect back to the
 * roles listing rather than /login: the admin layout has already let this
 * caller in once, so a typed error here is more likely a stale session
 * than an unauthenticated request.
 */
function redirectOnAuthError(err: unknown): never {
  const msg = err instanceof Error ? err.message : String(err);
  if (msg.includes("UNAUTHENTICATED") || msg.includes("FORBIDDEN")) redirect("/admin/roles");
  throw err;
}

export default async function EditRolePage({ params }: Props) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[admin/roles/:id/edit] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  const { id } = await params;

  let data: AdminRoleQueryType | null = null;
  try {
    data = await gqlFetch(AdminRoleQuery, { variables: { id }, revalidate: 0 });
  } catch (err) {
    redirectOnAuthError(err);
  }

  if (!data?.role) redirect("/admin/roles");

  const role = data.role as unknown as RoleForEdit;
  return <EditRoleClient role={role} />;
}
