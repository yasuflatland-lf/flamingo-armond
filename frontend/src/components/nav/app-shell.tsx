import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { GlobalRail } from "./global-rail";
import { LogoDrawer } from "./logo-drawer";

interface AppShellProps {
  /** Required user record. Callers must pass a value or explicit null. */
  user: { email: string | null } | null;
  /** Required admin flag — callers must explicitly pass false for non-admins. */
  isAdmin: boolean;
  children: React.ReactNode;
}

/**
 * Responsive application shell that mounts the navigation rail on PC (md+) and
 * a logo-triggered drawer on mobile (<md). The <SidebarProvider> lives here so
 * the rail and drawer share the same sidebar context.
 *
 * AppShell carries no "use client" directive of its own, but it is rendered
 * within the client boundary established by ConditionalShell (which reads
 * usePathname() to decide whether to mount the shell on the active route), so
 * it and its client children — GlobalRail, LogoDrawer, SidebarProvider — run on
 * the client. The page content reaches it through the `children` prop, so it
 * stays server-rendered. Keep this file free of any local state: AppShell is a
 * pure presentational composition, and all interactive state belongs in the
 * client nav components it mounts.
 */
export function AppShell({ user, isAdmin, children }: AppShellProps) {
  return (
    <SidebarProvider>
      {/* PC layout (md+): GlobalRail is a direct child of SidebarProvider so the
          Sidebar gap div participates in the flex row and offsets SidebarInset.
          The wrapper is hidden on mobile; the Sidebar component handles mobile
          display internally via a Sheet overlay. */}
      <aside
        data-testid="rail-container"
        className="hidden md:flex"
        aria-label="Primary navigation"
      >
        <GlobalRail user={user} isAdmin={isAdmin} />
      </aside>

      {/* Mobile layout (<md): top bar + logo-drawer trigger */}
      <div className="flex flex-1 flex-col">
        <header
          data-testid="mobile-header"
          className="md:hidden flex items-center justify-between px-4 h-12 border-b"
        >
          <LogoDrawer user={user} isAdmin={isAdmin} />
        </header>

        <SidebarInset>{children}</SidebarInset>
      </div>
    </SidebarProvider>
  );
}
