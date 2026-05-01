import { redirect } from "next/navigation";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { LoginButton } from "./login-button";

type SearchParams = Promise<{ error?: string }>;

export default async function LoginPage({ searchParams }: { searchParams: SearchParams }) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  if (authErr && authErr.name !== "AuthSessionMissingError") throw authErr;
  if (user) redirect("/cardgroups");

  const { error } = await searchParams;
  return (
    <main className="flex min-h-screen items-center justify-center p-8">
      <div className="flex flex-col items-center gap-4">
        <h1 className="text-2xl font-semibold">Sign in</h1>
        {error ? <p className="text-sm text-destructive">Sign-in failed: {error}</p> : null}
        <LoginButton />
      </div>
    </main>
  );
}
