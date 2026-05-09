import { redirect } from "next/navigation";
import { isIgnorableAuthError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { LoginButton } from "./login-button";

type SearchParams = Promise<{ error?: string }>;

export default async function LoginPage({ searchParams }: { searchParams: SearchParams }) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. For the login page, stale session means user is
  // null — no redirect to /cardgroups, so the page renders normally.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[login] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (user) redirect("/cardgroups");

  const { error } = await searchParams;
  return (
    <main data-testid="login-grid" className="relative grid h-svh lg:grid-cols-2">
      <div
        data-testid="brand-panel"
        className="max-lg:hidden flex flex-col items-center justify-center gap-6 bg-brand-tint"
      >
        <span className="text-7xl" role="img" aria-label="Flamingo">
          🦩
        </span>
        <div className="flex flex-col items-center gap-1 text-center">
          <span className="text-2xl font-semibold text-brand-tint-foreground">flamingo-armond</span>
          <span className="text-sm text-brand-tint-foreground/80">Remember more, study less.</span>
        </div>
      </div>

      <div className="flex flex-col">
        <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8">
          <h1 className="text-2xl font-semibold">Sign in</h1>
          {error ? <p className="text-sm text-destructive">Sign-in failed: {error}</p> : null}
          <LoginButton />
        </div>
        <footer className="pb-6 px-8 text-center text-xs text-muted-foreground">
          By signing in, you agree to our{" "}
          <a href="/terms" className="underline underline-offset-2 hover:text-foreground">
            Terms of Service
          </a>{" "}
          and{" "}
          <a href="/privacy" className="underline underline-offset-2 hover:text-foreground">
            Privacy Policy
          </a>
          .
        </footer>
      </div>
    </main>
  );
}
