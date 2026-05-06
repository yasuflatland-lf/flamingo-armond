import { redirect } from "next/navigation";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { ChangeEmailClient } from "./change-email-client";

export default async function ChangeEmailPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError is the "no session" signal — fall through to the
  // !user redirect below. Any other auth error is a real failure.
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[change-email] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">Change email</h1>
      <ChangeEmailClient currentEmail={user.email ?? null} />
    </main>
  );
}
