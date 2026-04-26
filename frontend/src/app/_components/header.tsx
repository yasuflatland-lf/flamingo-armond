import Link from "next/link";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { LogoutButton } from "./logout-button";

export async function Header() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error,
  } = await supabase.auth.getUser();
  // "AuthSessionMissingError" is the normal anonymous case — fall through to
  // the "Sign in" branch below. Only a different auth-service error means we
  // genuinely cannot tell the user's state, in which case we degrade by
  // hiding both the email and the sign-in link to avoid misleading them.
  if (error && error.name !== "AuthSessionMissingError") {
    console.error("[header] getUser() failed:", error.message);
    return (
      <header className="flex items-center justify-between border-b px-6 py-3">
        <Link href="/" className="font-semibold">
          🦩 flamingo-armond
        </Link>
      </header>
    );
  }

  return (
    <header className="flex items-center justify-between border-b px-6 py-3">
      <Link href="/" className="font-semibold">
        🦩 flamingo-armond
      </Link>
      <nav>
        {user ? (
          <div className="flex items-center gap-3 text-sm">
            <Link
              href="/profile"
              className="text-muted-foreground underline-offset-4 hover:underline"
            >
              {user.email}
            </Link>
            <LogoutButton />
          </div>
        ) : (
          <Link href="/login" className="text-sm underline">
            Sign in
          </Link>
        )}
      </nav>
    </header>
  );
}
