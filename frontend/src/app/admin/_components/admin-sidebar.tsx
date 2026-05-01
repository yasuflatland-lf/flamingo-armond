"use client";

import { BookOpen, ShieldCheck, Users } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

/**
 * Sidebar nav for the unified admin layout. Active highlight is computed from
 * `usePathname()` so every navigation re-renders the correct item without
 * round-tripping the server. Labels are kept in English per
 * `.claude/rules/language-policy.md`.
 *
 * The matcher accepts both an exact path match and a nested-route prefix
 * (e.g., `/admin/users/123/edit` highlights "Users") so deep links keep the
 * correct item active.
 */
const NAV_ITEMS = [
  { href: "/admin/users", label: "Users", icon: Users },
  { href: "/admin/roles", label: "Roles", icon: ShieldCheck },
  { href: "/admin/dictionary", label: "Dictionary", icon: BookOpen },
] as const;

export function AdminSidebar() {
  const pathname = usePathname();

  return (
    <aside
      aria-label="Admin navigation"
      className="w-56 shrink-0 border-r border-border bg-muted/40 p-4"
    >
      <nav>
        <ul className="space-y-1">
          {NAV_ITEMS.map((item) => {
            const Icon = item.icon;
            const isActive =
              pathname != null &&
              (pathname === item.href || pathname.startsWith(`${item.href}/`));
            return (
              <li key={item.href}>
                <Link
                  href={item.href}
                  aria-current={isActive ? "page" : undefined}
                  className={cn(
                    "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
                    isActive
                      ? "bg-accent text-accent-foreground"
                      : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                  <span>{item.label}</span>
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
    </aside>
  );
}
