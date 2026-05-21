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
      <div className="flex flex-col">
        <div
          data-testid="form-brand-header"
          className="lg:hidden flex items-center justify-center gap-2 pt-8"
        >
          <span className="text-2xl" role="img" aria-label="Flamingo">
            🦩
          </span>
          <span className="text-base font-medium">flamingo-armond</span>
        </div>

        <div className="flex flex-1 items-center justify-center p-8">
          <div className="w-full max-w-sm flex flex-col gap-6">
            <div className="flex flex-col gap-2 text-start">
              <h1 className="text-2xl font-semibold tracking-tight">Sign in</h1>
              <p className="text-sm text-muted-foreground">
                Sign in with your Google account to continue. New here? An account is created on
                first sign-in.
              </p>
            </div>

            {error ? (
              <p role="alert" className="text-sm text-destructive">
                Sign-in failed: {error}
              </p>
            ) : null}

            <LoginButton />

            <footer className="text-xs text-muted-foreground text-center">
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
        </div>
      </div>

      <div
        data-testid="brand-panel"
        className="max-lg:hidden relative flex flex-col items-center justify-center gap-4 overflow-hidden bg-gradient-to-br from-brand-tint via-brand-tint to-brand-tint/70"
      >
        <span className="text-8xl drop-shadow-sm" role="img" aria-label="Flamingo">
          🦩
        </span>
        <div className="flex flex-col items-center gap-1 text-center">
          <span className="text-3xl font-semibold tracking-tight text-brand-tint-foreground">
            flamingo-armond
          </span>
          <span className="text-sm text-brand-tint-foreground/80">Remember more, study less.</span>
        </div>
      </div>
    </main>
  );
}
