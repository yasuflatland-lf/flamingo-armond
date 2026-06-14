"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { AuthShell } from "./auth-shell";
import { AppleInstallHint } from "./pwa/apple-install-hint";

/**
 * Routes that own the entire viewport and must render without the navigation
 * shell. /login is the sign-in screen, /onboarding the display-name gate,
 * /onboarding/start the first-deck chooser (a deckless user's rail would point
 * at empty destinations), and /terms + /privacy the public legal pages (each
 * renders its own `<main>`).
 */
const BARE_ROUTES = new Set(["/login", "/onboarding", "/onboarding/start", "/terms", "/privacy"]);

interface ConditionalShellProps {
  /** Shell display identity, or null for the anonymous shell. */
  user: { email: string | null } | null;
  /** UI hint that gates the admin nav link. Authorization is enforced by app/admin/layout.tsx. */
  isAdmin: boolean;
  children: ReactNode;
}

/**
 * Client component that decides whether to mount the navigation shell based on
 * the current pathname.
 *
 * This MUST be a client component reading usePathname() rather than a
 * server-side branch in the root layout: the root layout is a server component
 * that does NOT re-render on client-side (soft) navigations, so a server-side
 * branch leaves the shell from the previously-rendered route in place. A
 * concrete repro: /login and the public /terms + /privacy pages are all
 * full-screen bare routes; the user opens /terms from the sign-in screen and
 * navigates back via its in-page `<Link href="/login">`. With the decision
 * frozen in the server layout the shell would stay mounted from whichever route
 * triggered the last full document load, stranding the nav rail over the bare
 * screen. usePathname() re-renders here on every navigation, so the shell is
 * correctly dropped on every bare route regardless of how it was reached.
 */
export function ConditionalShell({ user, isAdmin, children }: ConditionalShellProps) {
  const pathname = usePathname();

  if (BARE_ROUTES.has(pathname)) {
    return <>{children}</>;
  }

  return (
    <>
      <AuthShell user={user} isAdmin={isAdmin}>
        {children}
      </AuthShell>
      {/* AppleInstallHint calls useTranslations("Pwa"), so it MUST render inside
          NextIntlClientProvider — its ancestor in the root layout. Gated to the
          full-shell routes so the install banner never covers the sign-in or
          onboarding screens. */}
      <AppleInstallHint />
    </>
  );
}
