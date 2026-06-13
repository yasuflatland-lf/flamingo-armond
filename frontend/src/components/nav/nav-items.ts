import { Library, ShieldCheck, Users } from "lucide-react";
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
