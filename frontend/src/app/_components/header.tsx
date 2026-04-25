import Link from "next/link";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { LogoutButton } from "./logout-button";

export async function Header() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error,
  } = await supabase.auth.getUser();
  if (error) {
    // Auth service hiccup: render a degraded header rather than misleading
    // the user with either an email they aren't logged in as or a sign-in
    // link they can't actually use right now.
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
