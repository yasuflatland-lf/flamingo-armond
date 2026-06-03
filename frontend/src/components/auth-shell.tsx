import type { ReactNode } from "react";
import { AppShell } from "@/components/nav/app-shell";
import { Toaster } from "@/components/ui/sonner";

type AuthShellProps = {
  /** Shell display identity, or null for the anonymous shell. */
  user: { email: string | null } | null;
  /** UI hint that gates the admin nav link. Authorization is enforced by app/admin/layout.tsx. */
  isAdmin: boolean;
  children: ReactNode;
};

/**
 * Thin, synchronous navigation-shell wrapper. Identity is resolved once by the
 * middleware (forwarded via request headers) and read in app/layout.tsx, then
 * passed in here — this component performs no auth I/O, so it never suspends.
 */
export function AuthShell({ user, isAdmin, children }: AuthShellProps) {
  return (
    <AppShell user={user} isAdmin={isAdmin}>
      {children}
      <Toaster />
    </AppShell>
  );
}
