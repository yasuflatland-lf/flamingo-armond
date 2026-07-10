import { BookOpen, ChartColumn, Library, LibraryBig, ShieldCheck, User, Users } from "lucide-react";
import type { ComponentType } from "react";

export type AdminNavItem = {
  href: "/admin/users" | "/admin/roles" | "/admin/masters";
  /** Key into the `Nav` message namespace — resolved via `useTranslations("Nav")`. */
  labelKey: "users" | "roles" | "masters";
  icon: ComponentType<{ className?: string }>;
};

export const ADMIN_NAV_ITEMS: readonly AdminNavItem[] = [
  { href: "/admin/users", labelKey: "users", icon: Users },
  { href: "/admin/roles", labelKey: "roles", icon: ShieldCheck },
  { href: "/admin/masters", labelKey: "masters", icon: Library },
] as const;

export type CoreNavItem = {
  href: "/cardgroups" | "/catalog" | "/stats";
  /** Key into the `Nav` message namespace — resolved via `useTranslations("Nav")`. */
  labelKey: "cardgroups" | "catalog" | "progress";
  icon: ComponentType<{ className?: string }>;
};

/**
 * Center nav destinations shared by `GlobalRail` and `LogoDrawer`. Each consumer
 * keeps its own JSX shell (the rail wraps these in `SidebarMenuButton` + tooltip;
 * the drawer wraps them in `SheetClose` + `NAV_LINK_CLASS`); only the
 * `{ href, labelKey, icon }` triple is shared, so adding a destination is a
 * one-file change.
 */
export const CORE_NAV_ITEMS: readonly CoreNavItem[] = [
  { href: "/cardgroups", labelKey: "cardgroups", icon: BookOpen },
  { href: "/catalog", labelKey: "catalog", icon: LibraryBig },
  { href: "/stats", labelKey: "progress", icon: ChartColumn },
] as const;

export type FooterNavItem = {
  href: "/profile";
  /** Key into the `Nav` message namespace — resolved via `useTranslations("Nav")`. */
  labelKey: "profile";
  icon: ComponentType<{ className?: string }>;
};

/** Footer nav destinations shared by `GlobalRail` and `LogoDrawer`. */
export const FOOTER_NAV_ITEMS: readonly FooterNavItem[] = [
  { href: "/profile", labelKey: "profile", icon: User },
] as const;

/**
 * Active center-nav item resolved from the current pathname.
 *
 * Only handles the static center items (Cardgroups, Catalog, Progress). Admin
 * items and the footer Profile link compute their own active state inline, so
 * this type deliberately does not include `"profile"` or `"admin"`.
 */
export type ActiveItem = "cardgroups" | "catalog" | "progress" | null;

/** Matches `pathname` against a top-level route — exact match or a sub-route prefix. */
export function matchesRoute(pathname: string, route: string): boolean {
  return pathname === route || pathname.startsWith(`${route}/`);
}

/**
 * Resolve the active center-nav item from the current pathname.
 *
 * Uses a positive-allowlist style (per
 * `docs/frontend/typescript-conventions.md` § "Positive allowlist over
 * negative exclusion") so that future top-level routes do not silently match an
 * existing nav item.
 */
export function resolveActiveItem(pathname: string): ActiveItem {
  if (
    pathname === "/cardgroups" ||
    pathname.startsWith("/cardgroups/") ||
    pathname.startsWith("/cards/") ||
    pathname.startsWith("/learn/")
  ) {
    return "cardgroups";
  }
  if (matchesRoute(pathname, "/catalog")) {
    return "catalog";
  }
  if (matchesRoute(pathname, "/stats")) {
    return "progress";
  }
  return null;
}
