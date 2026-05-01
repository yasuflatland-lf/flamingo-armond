import { ShieldCheck } from "lucide-react";
import Link from "next/link";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { LogoutButton } from "./logout-button";
import { HeaderMeQuery } from "./queries";

export async function Header() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error,
  } = await supabase.auth.getUser();

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

  // Admin role check happens only when authenticated. Skipping the GraphQL
  // call for anonymous users avoids unnecessary backend traffic and stops
  // UNAUTHENTICATED noise from polluting the warn log.
  let isAdmin = false;
  if (user) {
    try {
      const meData = await gqlFetch(HeaderMeQuery, { revalidate: 0 });
      isAdmin = meData.me?.roles.some((r) => r.name === "admin") ?? false;
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes("UNAUTHENTICATED")) {
        // Expected race: Supabase session is valid but the GraphQL endpoint
        // rejected the access token (clock skew, JWKS rotation gap, etc.).
        // The admin layout will redirect on actual /admin navigation, so the
        // header degrades silently here.
      } else {
        console.warn("[header] me query unexpectedly failed", {
          error_message: msg,
          user_id: user.id,
        });
      }
    }
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
              href="/cardgroups"
              className="text-muted-foreground underline-offset-4 hover:underline"
            >
              Cardgroups
            </Link>
            {isAdmin ? (
              <Link
                href="/admin"
                className="flex items-center gap-1 text-muted-foreground underline-offset-4 hover:underline"
              >
                <ShieldCheck className="h-4 w-4" aria-hidden="true" />
                Admin
              </Link>
            ) : null}
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
