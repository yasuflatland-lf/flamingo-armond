"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { AuthShell } from "./auth-shell";
import { AppleInstallHint } from "./pwa/apple-install-hint";

/**
 * Routes that own the entire viewport and must render without the navigation
 * shell. /login is the sign-in screen; /onboarding is the display-name gate.
 */
const BARE_ROUTES = new Set(["/login", "/onboarding"]);

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
 * This MUST be a client component reading usePathname() rather than a server
 * branch on the middleware-forwarded x-pathname header: the root layout is a
 * server component that does NOT re-render on client-side (soft) navigations,
 * so a server-side branch leaves the shell from the previously-rendered route
 * in place. A concrete repro: from /login a full-page link to /terms (a 404)
 * SSRs the full shell; the not-found page's "Back to Home" link soft-navigates
 * to / which redirects to /login, and the shell — rail included — lingers over
 * the sign-in screen. usePathname() re-renders here on every navigation, so the
 * shell is correctly dropped on bare routes regardless of how the route was
 * reached.
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
