import type { Metadata } from "next";
import { headers } from "next/headers";
import type { ReactNode } from "react";
import { AppShell } from "@/components/nav/app-shell";
import { GlobalFAB } from "@/components/nav/global-fab";
import { Toaster } from "@/components/ui/sonner";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { HeaderMeQuery } from "./_components/queries";
import { Providers } from "./providers";
import "./globals.css";

export const metadata: Metadata = {
  title: "flamingo-armond",
  description: "Swiping flashcard app.",
};

export default async function RootLayout({ children }: { children: ReactNode }) {
  // Read the pathname forwarded by middleware so this server component can
  // decide whether to mount the navigation shell. /login renders bare so the
  // sign-in screen owns the entire viewport.
  const headersList = await headers();
  const pathname = headersList.get("x-pathname") ?? "/";

  if (pathname === "/login" || pathname === "/onboarding") {
    return (
      <html lang="en">
        <body suppressHydrationWarning>
          <Providers>{children}</Providers>
        </body>
      </html>
    );
  }

  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error,
  } = await supabase.auth.getUser();

  // AuthSessionMissingError is the normal anonymous-request signal — anything
  // else from getUser() is a real failure that must degrade the shell to null.
  const getUserFailed = error != null && error.name !== "AuthSessionMissingError";
  if (getUserFailed) {
    console.error("[layout] getUser() failed:", error.name, error.message);
  }

  // Skip the GraphQL me-query when the user is anonymous: the backend would
  // return UNAUTHENTICATED and the warn log would fill with expected noise.
  let isAdmin = false;
  if (user && !getUserFailed) {
    try {
      const meData = await gqlFetch(HeaderMeQuery, { revalidate: 0 });
      isAdmin = meData.me?.roles.some((r) => r.name === "admin") ?? false;
    } catch (err) {
      // Expected race: Supabase session valid but GraphQL token rejected
      // (clock skew, JWKS rotation gap). The /admin layout redirects on
      // actual navigation, so the header degrades silently here.
      if (!isUnauthenticatedGraphQLError(err)) {
        console.warn("[layout] me query unexpectedly failed", {
          error_message: err instanceof Error ? err.message : String(err),
          user_id: user.id,
        });
      }
    }
  }

  // Degrade to null user on a real getUser() error so AppShell renders the
  // anonymous shell rather than an authenticated shell with potentially stale data.
  const shellUser = getUserFailed || !user ? null : { email: user.email ?? null };

  return (
    <html lang="en">
      {/*
        Browser extensions (ColorZilla, Grammarly, etc.) inject attributes onto
        <body> before React hydrates, which causes a benign hydration mismatch.
        suppressHydrationWarning is shallow (this element only) so real hydration
        bugs in children still surface.
      */}
      <body suppressHydrationWarning>
        <Providers>
          <AppShell user={shellUser} isAdmin={isAdmin}>
            {children}
            <GlobalFAB />
            {/*
              Toaster must live inside AppShell (a client-boundary component) because
              sonner requires a client rendering context. Placing it here ensures the
              toast container is mounted for every authenticated route while remaining
              inside the client boundary. RSC + sonner is incompatible at the layout level.
            */}
            <Toaster />
          </AppShell>
        </Providers>
      </body>
    </html>
  );
}
