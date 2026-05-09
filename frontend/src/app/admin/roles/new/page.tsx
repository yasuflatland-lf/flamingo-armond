import { redirect } from "next/navigation";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { NewRoleClient } from "./new-role-client";

/**
 * RSC for the role creation page.
 *
 * admin/layout.tsx is the source-of-truth admin gate; the per-page getUser
 * here is defense-in-depth (mirroring admin/users/[id]/edit/page.tsx).
 */
export default async function NewRolePage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[admin/roles/new] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  return <NewRoleClient />;
}
