import { redirect } from "next/navigation";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";

export default async function SettingsPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[settings] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">Settings</h1>
      <div className="rounded-lg border border-input bg-background p-4">
        <h2 className="font-semibold">Coming soon.</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Settings will be available in a future update.
        </p>
      </div>
    </main>
  );
}
