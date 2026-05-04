import type { Metadata } from "next";
import type { ReactNode } from "react";
import { AppShell } from "@/components/nav/app-shell";
import { GlobalFAB } from "@/components/nav/global-fab";
import { HeaderMeQuery } from "./_components/queries";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { Providers } from "./providers";
import "./globals.css";

export const metadata: Metadata = {
  title: "flamingo-armond",
  description: "Swiping flashcard app.",
};

export default async function RootLayout({ children }: { children: ReactNode }) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error,
  } = await supabase.auth.getUser();

  if (error && error.name !== "AuthSessionMissingError") {
    console.error("[layout] getUser() failed:", error.name, error.message);
  }

  // Skip the GraphQL me-query when the user is anonymous: the backend would
  // return UNAUTHENTICATED and the warn log would fill with expected noise.
  let isAdmin = false;
  if (user && !(error && error.name !== "AuthSessionMissingError")) {
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
  const shellUser =
    error && error.name !== "AuthSessionMissingError"
      ? null
      : user
        ? { email: user.email ?? null }
        : null;

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
          </AppShell>
        </Providers>
      </body>
    </html>
  );
}
