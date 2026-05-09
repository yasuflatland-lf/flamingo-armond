import { redirect } from "next/navigation";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { ChangeEmailClient } from "./change-email-client";

export default async function ChangeEmailPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[change-email] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">Change email</h1>
      <ChangeEmailClient currentEmail={user.email ?? null} />
    </main>
  );
}
