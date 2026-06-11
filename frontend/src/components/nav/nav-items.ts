import { ShieldCheck, Users } from "lucide-react";
import type { ComponentType } from "react";

export type AdminNavItem = {
  href: "/admin/users" | "/admin/roles";
  /** Key into the `Nav` message namespace — resolved via `useTranslations("Nav")`. */
  labelKey: "users" | "roles";
  icon: ComponentType<{ className?: string }>;
};

export const ADMIN_NAV_ITEMS: readonly AdminNavItem[] = [
  { href: "/admin/users", labelKey: "users", icon: Users },
  { href: "/admin/roles", labelKey: "roles", icon: ShieldCheck },
] as const;
