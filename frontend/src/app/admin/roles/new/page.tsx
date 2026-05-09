import { redirect } from "next/navigation";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
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
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[admin/roles/new] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  return <NewRoleClient />;
}
