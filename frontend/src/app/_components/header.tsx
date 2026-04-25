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
    // Degrade gracefully on auth-service errors: showing email or a sign-in
    // link could mislead the user about their actual session state.
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
