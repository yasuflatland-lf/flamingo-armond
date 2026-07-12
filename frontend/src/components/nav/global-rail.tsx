"use client";

import { LogOut } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useRef } from "react";
import { useLogout } from "@/app/_components/use-logout";
import { FlamingoMark } from "@/components/brand/flamingo-mark";
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
import {
  ADMIN_NAV_ITEMS,
  CORE_NAV_ITEMS,
  FOOTER_NAV_ITEMS,
  matchesRoute,
  resolveActiveItem,
} from "./nav-items";

interface GlobalRailProps {
  /** Required user record. Callers must pass a value or explicit null. */
  user: { email: string | null } | null;
  /** Required admin flag — callers must explicitly pass false for non-admins. */
  isAdmin: boolean;
}

/** Hover-out close delay (ms) — avoids flicker when crossing the rail boundary. */
const HOVER_CLOSE_DELAY_MS = 150;

export function GlobalRail({ user, isAdmin }: GlobalRailProps) {
  const t = useTranslations("Nav");
  const pathname = usePathname();
  const { state, setOpen, isMobile } = useSidebar();
  const logout = useLogout();

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
        <div className="flex size-8 items-center justify-center">
          <Link href="/" aria-label={t("flamingoHome")} className="shrink-0 leading-none">
            <FlamingoMark className="size-7" aria-hidden="true" />
          </Link>
        </div>
      </SidebarHeader>

      {user !== null && (
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                {CORE_NAV_ITEMS.map((item) => {
                  const isItemActive = active === item.labelKey;
                  const Icon = item.icon;
                  return (
                    <SidebarMenuItem key={item.href}>
                      <SidebarMenuButton asChild isActive={isItemActive} tooltip={t(item.labelKey)}>
                        <Link href={item.href} aria-current={isItemActive ? "page" : undefined}>
                          <Icon aria-hidden="true" />
                          <span>{t(item.labelKey)}</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}

                {isAdmin &&
                  ADMIN_NAV_ITEMS.map((item) => {
                    const isItemActive =
                      matchesRoute(pathname, item.href) ||
                      (item.href === "/admin/users" && fallbackToUsers);
                    const Icon = item.icon;
                    return (
                      <SidebarMenuItem key={item.href}>
                        <SidebarMenuButton
                          asChild
                          isActive={isItemActive}
                          tooltip={t(item.labelKey)}
                        >
                          <Link href={item.href} aria-current={isItemActive ? "page" : undefined}>
                            <Icon aria-hidden="true" />
                            <span>{t(item.labelKey)}</span>
                          </Link>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    );
                  })}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
      )}

      {user !== null && (
        <SidebarFooter>
          <SidebarMenu>
            {/*
              Footer item active state is computed inline via `matchesRoute`
              because these links are not part of `resolveActiveItem`'s
              center-item domain.
            */}
            {FOOTER_NAV_ITEMS.map((item) => {
              const isItemActive = matchesRoute(pathname, item.href);
              const Icon = item.icon;
              return (
                <SidebarMenuItem key={item.href}>
                  <SidebarMenuButton asChild isActive={isItemActive} tooltip={t(item.labelKey)}>
                    <Link href={item.href} aria-current={isItemActive ? "page" : undefined}>
                      <Icon aria-hidden="true" />
                      <span>{t(item.labelKey)}</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              );
            })}

            {/*
              Logout is an action, not a nav link, so it renders a <button>
              (no `asChild`/`Link`) — but as a SidebarMenuButton it stays a
              pixel-for-pixel peer of the Profile row above (same padding, gap,
              height, hover background) and collapses to an icon + tooltip with
              the rest of the rail. Sign-out logic is shared with the drawer's
              LogoutButton via useLogout().
            */}
            <SidebarMenuItem>
              <SidebarMenuButton type="button" onClick={logout} tooltip={t("logout")}>
                <LogOut aria-hidden="true" />
                <span>{t("logout")}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
      )}

      {user === null && pathname !== "/login" && (
        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton asChild tooltip={t("signIn")}>
                <HeaderSignInLink />
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
      )}
    </Sidebar>
  );
}
