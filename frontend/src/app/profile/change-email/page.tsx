import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { ChangeEmailClient } from "./change-email-client";

export default async function ChangeEmailPage() {
  const auth = readAuthContext(await headers());
  if (auth.status !== "authenticated") redirect("/login");

  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">Change email</h1>
      <ChangeEmailClient currentEmail={auth.email} />
    </main>
  );
}
