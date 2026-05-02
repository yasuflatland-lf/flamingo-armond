import { BookOpen } from "lucide-react";
import Link from "next/link";
import { LogoutButton } from "@/app/_components/logout-button";
import { HeaderMeQuery } from "@/app/_components/queries";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import AdminPill from "./admin-pill";
import { HamburgerDrawer } from "./hamburger-drawer";
import { HeaderAddCardLink } from "./header-add-card-link";

export async function GlobalHeader() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error,
  } = await supabase.auth.getUser();

  if (error && error.name !== "AuthSessionMissingError") {
    console.error("[global-header] getUser() failed:", error.name, error.message);
    // Degrade to logo-only shell — never throw from the root-layout header.
    return (
      <header className="flex items-center justify-between border-b px-4 py-3">
        <Link href="/" className="font-semibold">
          🦩 flamingo-armond
        </Link>
      </header>
    );
  }

  // Skip the GraphQL me-query when the user is anonymous: the backend would
  // return UNAUTHENTICATED and the warn log would fill with expected noise.
  let isAdmin = false;
  if (user) {
    try {
      const meData = await gqlFetch(HeaderMeQuery, { revalidate: 0 });
      isAdmin = meData.me?.roles.some((r) => r.name === "admin") ?? false;
    } catch (err) {
      // Expected race: Supabase session valid but GraphQL token rejected
      // (clock skew, JWKS rotation gap). The /admin layout redirects on
      // actual navigation, so the header degrades silently here.
      if (!isUnauthenticatedGraphQLError(err)) {
        console.warn("[global-header] me query unexpectedly failed", {
          error_message: err instanceof Error ? err.message : String(err),
          user_id: user.id,
        });
      }
    }
  }

  return (
    <header className="flex items-center justify-between border-b px-4 py-3">
      {/* ── Mobile layout (< md) ─────────────────────────────────── */}
      <div className="flex items-center gap-3 md:hidden">
        {user && <HamburgerDrawer isAdmin={isAdmin} userEmail={user.email} />}
        <Link href="/" className="font-semibold">
          🦩 flamingo-armond
        </Link>
      </div>

      {/* Mobile: right-aligned email (shown only when signed in) */}
      {user && (
        <span className="ml-auto truncate max-w-[160px] text-xs text-muted-foreground md:hidden">
          {user.email}
        </span>
      )}

      {/* Mobile: sign-in link when anonymous */}
      {!user && (
        <Link href="/login" className="text-sm underline md:hidden">
          Sign in
        </Link>
      )}

      {/* ── Desktop layout (≥ md) ─────────────────────────────────── */}
      <div className="hidden md:flex md:items-center md:gap-4 md:w-full">
        <Link href="/" className="font-semibold shrink-0">
          🦩 flamingo-armond
        </Link>

        {user ? (
          <>
            <nav className="flex items-center gap-3 text-sm ml-2">
              <Link
                href="/cardgroups"
                className="flex items-center gap-1.5 text-muted-foreground underline-offset-4 hover:underline"
              >
                <BookOpen className="h-4 w-4" aria-hidden="true" />
                Cardgroups
              </Link>

              <HeaderAddCardLink />
            </nav>

            <div className="flex items-center gap-3 text-sm ml-auto">
              <Link
                href="/profile"
                className="text-muted-foreground underline-offset-4 hover:underline truncate max-w-[200px]"
              >
                {user.email}
              </Link>
              {isAdmin && <AdminPill />}
              <LogoutButton />
            </div>
          </>
        ) : (
          <div className="ml-auto">
            <Link href="/login" className="text-sm underline">
              Sign in
            </Link>
          </div>
        )}
      </div>
    </header>
  );
}
