"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { AppShell } from "@/components/nav/app-shell";
import { Toaster } from "@/components/ui/sonner";
import { AppleInstallHint } from "./pwa/apple-install-hint";

/**
 * Route prefixes that DO mount the navigation shell — the authenticated,
 * onboarded user's in-app content sections. The shell is gated by this
 * allowlist rather than a denylist of bare routes, so any path that is NOT a
 * known content route renders bare. Why the allowlist direction matters:
 *
 * - Pre-auth / pre-onboarding / legal screens (`/login`, `/onboarding`,
 *   `/onboarding/start`, `/terms`, `/privacy`) own the full viewport and carry
 *   no nav chrome — they simply are not in this list.
 * - `/` is a redirect-only dispatcher (`app/page.tsx` always `redirect()`s and
 *   never renders content — see docs/frontend/routing-topology.md). It is not a
 *   content route, so it stays bare; mounting the shell there would flash the
 *   nav rail for the brief moment `/` resolves and redirects, most visibly on a
 *   new user's first login (`/` → `/onboarding`), where the rail appears then
 *   vanishes.
 * - An unknown path renders `app/not-found.tsx` (404) under the SAME arbitrary
 *   pathname the user typed. A denylist could never enumerate every such path,
 *   so the 404 used to mount the full shell — surfacing the nav rail to a user
 *   who has not finished onboarding. Allowlisting content routes makes every
 *   unknown path bare by construction, which is the correct terminal-screen
 *   treatment (`not-found.tsx` owns its own `<main>` + "Back to Home" CTA).
 *
 * A 404 raised UNDER a content prefix (e.g. `/learn/<bad-id>`) intentionally
 * keeps the shell: that user is an onboarded caller who mistyped a real
 * section, and the rail is a useful escape hatch.
 *
 * 500-class errors are out of scope here: `app/error.tsx` paints a full-screen
 * `BrandSplash` over any mounted shell, and `app/global-error.tsx` replaces the
 * root layout entirely — neither leaves the rail visible.
 */
const SHELL_ROUTE_PREFIXES = [
  "/admin",
  "/cardgroups",
  "/cards",
  "/catalog",
  "/learn",
  "/profile",
] as const;

/**
 * True when `pathname` is one of the in-app content routes that mount the
 * navigation shell. Matches the exact prefix or a sub-path under it, with a `/`
 * boundary so `/cardgroups` matches `/cardgroups/123` but not `/cardgroupsX`.
 */
function isShellRoute(pathname: string): boolean {
  return SHELL_ROUTE_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  );
}

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

  if (!isShellRoute(pathname)) {
    return <>{children}</>;
  }

  return (
    <>
      <AppShell user={user} isAdmin={isAdmin}>
        {children}
        <Toaster />
      </AppShell>
      {/* AppleInstallHint calls useTranslations("Pwa"), so it MUST render inside
          NextIntlClientProvider — its ancestor in the root layout. Gated to the
          full-shell routes so the install banner never covers the sign-in or
          onboarding screens. */}
      <AppleInstallHint />
    </>
  );
}
