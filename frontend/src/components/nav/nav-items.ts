import { ShieldCheck, Users } from "lucide-react";
import type { ComponentType } from "react";

export type AdminNavItem = {
  href: "/admin/users" | "/admin/roles";
  label: "Users" | "Roles";
  icon: ComponentType<{ className?: string }>;
};

export const ADMIN_NAV_ITEMS: readonly AdminNavItem[] = [
  { href: "/admin/users", label: "Users", icon: Users },
  { href: "/admin/roles", label: "Roles", icon: ShieldCheck },
] as const;
