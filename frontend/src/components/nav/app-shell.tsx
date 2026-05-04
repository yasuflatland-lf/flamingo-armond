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
 * AppShell is a server component. GlobalRail, LogoDrawer, and SidebarProvider
 * are all client components — React Server Components composition rules allow
 * us to mount them as children from this server component. However, any state
 * added to AppShell itself must remain server-side, so keep the "use client"
 * directive off this file.
 */
export function AppShell({ user, isAdmin, children }: AppShellProps) {
  return (
    <SidebarProvider>
      {/* PC layout (md+): persistent rail on the left */}
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
          {user && user.email !== null && (
            <span className="text-sm text-muted-foreground truncate max-w-[160px]">
              {user.email}
            </span>
          )}
        </header>

        {/* Main content — SidebarInset handles the left-offset when the PC rail
            is visible, via peer CSS selectors in the Shadcn sidebar primitive. */}
        <SidebarInset>{children}</SidebarInset>
      </div>
    </SidebarProvider>
  );
}
