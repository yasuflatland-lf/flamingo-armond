import Link from "next/link";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { LogoutButton } from "./logout-button";

export async function Header() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();

  return (
    <header className="flex items-center justify-between border-b px-6 py-3">
      <Link href="/" className="font-semibold">
        🦩 flamingo-armond
      </Link>
      <nav>
        {user ? (
          <div className="flex items-center gap-3 text-sm">
            <span className="text-muted-foreground">{user.email}</span>
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
