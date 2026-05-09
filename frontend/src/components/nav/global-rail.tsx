"use client";

import { BookOpen, Settings, User } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";
import { LogoutButton } from "@/app/_components/logout-button";
import { SidebarToggle } from "@/components/nav/sidebar-toggle";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar";
import { HeaderSignInLink } from "./header-sign-in-link";
import { ADMIN_NAV_ITEMS } from "./nav-items";

interface GlobalRailProps {
  /** Required user record. Callers must pass a value or explicit null. */
  user: { email: string | null } | null;
  /** Required admin flag — callers must explicitly pass false for non-admins. */
  isAdmin: boolean;
}

/** Hover-out close delay (ms) — avoids flicker when crossing the rail boundary. */
const HOVER_CLOSE_DELAY_MS = 150;

/**
 * Resolve the active rail item from the current pathname.
 *
 * Only handles the static center items (Cardgroups, Settings). Admin items and
 * the footer Profile link compute their own active state inline, so this
 * function deliberately does not return `"profile"` or `"admin"`.
 *
 * Uses a positive-allowlist style (per
 * `docs/frontend/typescript-conventions.md` § "Positive allowlist over
 * negative exclusion") so that future top-level routes do not silently match an
 * existing rail item.
 */
type ActiveItem = "cardgroups" | "settings" | null;

/** Matches `pathname` against a top-level route — exact match or a sub-route prefix. */
function matchesRoute(pathname: string, route: string): boolean {
  return pathname === route || pathname.startsWith(`${route}/`);
}

function resolveActiveItem(pathname: string): ActiveItem {
  if (
    pathname === "/cardgroups" ||
    pathname.startsWith("/cardgroups/") ||
    pathname.startsWith("/cards/") ||
    pathname.startsWith("/learn/")
  ) {
    return "cardgroups";
  }
  if (pathname === "/settings") {
    return "settings";
  }
  return null;
}

export function GlobalRail({ user, isAdmin }: GlobalRailProps) {
  const pathname = usePathname();
  const { state, toggleSidebar, setOpen, isMobile } = useSidebar();

  // Hover-flyout close timer. Use useRef (not useState) to avoid an async update
  // dropping a pointerenter that arrives in the same frame as the timeout fires
  // (same hazard documented in docs/pagination/intersection-observer-in-flight-guard.md).
  const hoverCloseTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearHoverTimer = () => {
    if (hoverCloseTimerRef.current !== null) {
      clearTimeout(hoverCloseTimerRef.current);
      hoverCloseTimerRef.current = null;
    }
  };

  // Drop any pending timer on unmount so a delayed setOpen(false) does not fire
  // against a stale component.
  useEffect(() => {
    return () => {
      if (hoverCloseTimerRef.current !== null) {
        clearTimeout(hoverCloseTimerRef.current);
        hoverCloseTimerRef.current = null;
      }
    };
  }, []);

  const handlePointerEnter = () => {
    if (isMobile) return;
    clearHoverTimer();
    if (state === "collapsed") {
      setOpen(true);
    }
  };

  const handlePointerLeave = () => {
    if (isMobile) return;
    clearHoverTimer();
    hoverCloseTimerRef.current = setTimeout(() => {
      setOpen(false);
      hoverCloseTimerRef.current = null;
    }, HOVER_CLOSE_DELAY_MS);
  };

  const active = resolveActiveItem(pathname);

  // Footer Profile link active state — computed inline because the footer
  // Profile link is not part of `resolveActiveItem`'s center-item domain.
  const profileActive = matchesRoute(pathname, "/profile");

  // Admin fallback: light up Users when /admin/<unknown> falls through,
  // so the rail always points at a valid admin destination. Encoded with a
  // literal-string discriminator on `item.href`, not a numeric index — see
  // `docs/frontend/typescript-conventions.md` § "Positive allowlist".
  const matchedAnyAdmin = ADMIN_NAV_ITEMS.some((i) => matchesRoute(pathname, i.href));
  const fallbackToUsers = !matchedAnyAdmin && matchesRoute(pathname, "/admin");

  return (
    <Sidebar
      collapsible="icon"
      onPointerEnter={handlePointerEnter}
      onPointerLeave={handlePointerLeave}
    >
      <SidebarHeader>
        <div className="flex h-8 items-center gap-2 px-2">
          <Link href="/" aria-label="Flamingo home" className="shrink-0 text-lg leading-none">
            🦩
          </Link>
          <SidebarToggle expanded={state === "expanded"} onToggle={toggleSidebar} />
        </div>
      </SidebarHeader>

      {user !== null && (
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    asChild
                    isActive={active === "cardgroups"}
                    tooltip="Cardgroups"
                  >
                    <Link
                      href="/cardgroups"
                      aria-current={active === "cardgroups" ? "page" : undefined}
                    >
                      <BookOpen aria-hidden="true" />
                      <span>Cardgroups</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                {isAdmin &&
                  ADMIN_NAV_ITEMS.map((item) => {
                    const isItemActive =
                      matchesRoute(pathname, item.href) ||
                      (item.href === "/admin/users" && fallbackToUsers);
                    const Icon = item.icon;
                    return (
                      <SidebarMenuItem key={item.href}>
                        <SidebarMenuButton asChild isActive={isItemActive} tooltip={item.label}>
                          <Link href={item.href} aria-current={isItemActive ? "page" : undefined}>
                            <Icon aria-hidden="true" />
                            <span>{item.label}</span>
                          </Link>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    );
                  })}

                <SidebarMenuItem>
                  <SidebarMenuButton asChild isActive={active === "settings"} tooltip="Settings">
                    <Link
                      href="/settings"
                      aria-current={active === "settings" ? "page" : undefined}
                    >
                      <Settings aria-hidden="true" />
                      <span>Settings</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
      )}

      {user !== null && (
        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton asChild isActive={profileActive} tooltip="Profile">
                <Link href="/profile" aria-current={profileActive ? "page" : undefined}>
                  <User aria-hidden="true" />
                  <span>Profile</span>
                </Link>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>

          {/*
            Wrap LogoutButton in a div with the collapsed-hide class so the
            button label collapses with the rail. The class is intentionally on
            the wrapper, not on LogoutButton itself — that keeps LogoutButton
            uncoupled from the rail's collapsed-state CSS group.
          */}
          <div className="group-data-[collapsible=icon]:hidden">
            <LogoutButton />
          </div>
        </SidebarFooter>
      )}

      {user === null && pathname !== "/login" && (
        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton asChild tooltip="Sign in">
                <HeaderSignInLink />
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
      )}
    </Sidebar>
  );
}
