"use client";

import { BookOpen, GraduationCap, Settings, ShieldCheck, User } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";
import { LogoutButton } from "@/app/_components/logout-button";
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
 * Uses a positive-allowlist style (per
 * `.claude/rules/frontend-typescript-conventions.md` § "Positive allowlist over
 * negative exclusion") so that future top-level routes do not silently match an
 * existing rail item.
 */
type ActiveItem = "learning" | "cardgroups" | "profile" | "admin" | "settings" | null;

function resolveActiveItem(pathname: string): ActiveItem {
  if (pathname === "/learn" || pathname.startsWith("/learn/")) {
    return "learning";
  }
  if (
    pathname === "/cardgroups" ||
    pathname.startsWith("/cardgroups/") ||
    pathname.startsWith("/cards/")
  ) {
    return "cardgroups";
  }
  if (pathname === "/profile") {
    return "profile";
  }
  if (pathname === "/admin" || pathname.startsWith("/admin/")) {
    return "admin";
  }
  if (pathname === "/settings") {
    return "settings";
  }
  return null;
}

export function GlobalRail({ user, isAdmin }: GlobalRailProps) {
  const pathname = usePathname();
  const { state, setOpen, isMobile } = useSidebar();

  // Hover-flyout close timer. Use useRef (not useState) to avoid an async update
  // dropping a pointerenter that arrives in the same frame as the timeout fires
  // (same hazard documented in `.claude/rules/pagination.md` for the
  // IntersectionObserver in-flight guard).
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

  return (
    <Sidebar
      collapsible="icon"
      onPointerEnter={handlePointerEnter}
      onPointerLeave={handlePointerLeave}
    >
      <SidebarHeader>
        <div className="flex flex-col gap-2">
          <Link
            href="/cardgroups"
            aria-label="flamingo-armond home"
            className="flex h-12 items-center justify-center rounded-md hover:bg-sidebar-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
          >
            {/* unoptimized avoids requiring images.dangerouslyAllowSVG in next.config.ts */}
            <Image
              src="/flamingo.svg"
              alt="flamingo-armond"
              width={48}
              height={48}
              priority
              unoptimized
            />
          </Link>
          {user !== null && user.email !== null && (
            <p className="truncate px-2 text-xs text-muted-foreground group-data-[collapsible=icon]:hidden">
              {user.email}
            </p>
          )}
          {user !== null && (
            <div className="px-1 group-data-[collapsible=icon]:hidden">
              <LogoutButton />
            </div>
          )}
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
                    isActive={active === "learning"}
                    tooltip="Learning"
                  >
                    <Link
                      href="/learn"
                      aria-current={active === "learning" ? "page" : undefined}
                    >
                      <GraduationCap aria-hidden="true" />
                      <span>Learning</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

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

                <SidebarMenuItem>
                  <SidebarMenuButton asChild isActive={active === "profile"} tooltip="Profile">
                    <Link href="/profile" aria-current={active === "profile" ? "page" : undefined}>
                      <User aria-hidden="true" />
                      <span>Profile</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                {isAdmin && (
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={active === "admin"} tooltip="Admin">
                      <Link href="/admin" aria-current={active === "admin" ? "page" : undefined}>
                        <ShieldCheck aria-hidden="true" />
                        <span>Admin</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                )}

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
