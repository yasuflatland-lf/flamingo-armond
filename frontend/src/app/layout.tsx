import { SpeedInsights } from "@vercel/speed-insights/next";
import type { Metadata } from "next";
import { headers } from "next/headers";
import type { ReactNode } from "react";
import { AppShell } from "@/components/nav/app-shell";
import { GlobalFAB } from "@/components/nav/global-fab";
import { Toaster } from "@/components/ui/sonner";
import { createSupabaseServerClient } from "@/lib/supabase/server";
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
          <SpeedInsights />
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

  // Skip the getClaims call when the user is anonymous: no session means no
  // JWT to inspect. The /admin layout redirects on actual navigation, so the
  // header degrades to isAdmin=false for anonymous visitors without noise.
  let isAdmin = false;
  if (user && !getUserFailed) {
    const { data: claimsData, error: claimsError } = await supabase.auth.getClaims();
    if (claimsError != null) {
      // getClaims failures are non-fatal: the Supabase session is valid but
      // the JWT could not be validated locally (e.g. key rotation gap, cold
      // serverless instance without a cached JWKS). Degrade to isAdmin=false
      // and warn so the operator can correlate with JWT key rotation events.
      console.warn("[layout] getClaims() failed", {
        user_id: user.id,
        error_name: claimsError.name,
      });
    } else if (claimsData == null) {
      // Third return shape from the SDK: `{ data: null, error: null }` — the
      // TOCTOU race window where the session vanished between getUser() and
      // getClaims(). Degrade to isAdmin=false and warn so the operator can
      // correlate with session-revocation or sign-out events.
      console.warn("[layout] getClaims() returned null data without error", {
        user_id: user.id,
      });
    } else if (claimsData.claims == null) {
      // Fourth return shape that is not in the SDK's current TS types but is
      // forward-compatibility / mock-robustness defence: `{ data: { claims:
      // null }, error: null }`. The optional chain `.claims?.app_metadata` used
      // to silently degrade through this case; an explicit narrowing surfaces
      // it via a warn so an operator can correlate with future SDK type drift.
      console.warn("[layout] getClaims() returned data without claims", {
        user_id: user.id,
      });
    } else {
      isAdmin = claimsData.claims.app_metadata?.role === "admin";
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
        <SpeedInsights />
      </body>
    </html>
  );
}
