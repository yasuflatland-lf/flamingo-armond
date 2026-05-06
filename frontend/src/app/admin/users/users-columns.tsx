"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { MoreHorizontal } from "lucide-react";
import Link from "next/link";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTableColumnHeader } from "@/components/ui/data-table-column-header";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

/**
 * Row shape for the admin users DataTable. Fields map directly to the
 * AdminUserFields fragment, with roles inlined. All nullable fields use the
 * required-but-nullable form (`T | null`) rather than optional (`?: T | null`)
 * so callers must acknowledge each field's presence per the project convention.
 *
 * See: .claude/rules/frontend-typescript-conventions.md
 *   § "Required `string | null` over optional `?: string | null`"
 */
export type AdminUserRow = {
  id: string;
  displayName: string | null;
  avatarUrl: string | null;
  lastActive: string | null;
  roles: { id: string; name: string }[];
};

/**
 * Return the two-letter uppercase initials of a display name.
 * Takes the first character of each space-separated word, up to 2, uppercased.
 * Returns "?" when the name is null or empty.
 */
function getInitials(displayName: string | null): string {
  if (displayName === null || displayName.trim() === "") return "?";
  const parts = displayName.trim().split(/\s+/);
  return parts
    .slice(0, 2)
    .map((w) => w.charAt(0).toUpperCase())
    .join("");
}

/**
 * Format an ISO-8601 timestamp (or null) as a human-readable relative time
 * string. Uses `Date.now()` at render time for the current instant.
 *
 * Examples:
 *   null          → renders a "Never" span
 *   < 60 s ago   → "Just now"
 *   < 1 h ago    → "N minutes ago"
 *   < 24 h ago   → "N hours ago"
 *   < 30 d ago   → "N days ago"
 *   < 12 mo ago  → "N months ago"
 *   else          → "N years ago"
 */
function formatRelativeTime(lastActive: string | null): React.ReactNode {
  if (lastActive === null) {
    return <span className="text-muted-foreground">Never</span>;
  }

  const parsed = new Date(lastActive).getTime();
  if (Number.isNaN(parsed)) {
    console.warn("[admin-users] invalid lastActive timestamp", { length: lastActive.length });
    return <span className="text-muted-foreground italic">unknown</span>;
  }
  const diffSec = Math.floor((Date.now() - parsed) / 1000);
  if (diffSec < 60) return "Just now";

  const plural = (n: number, unit: string) => `${n} ${unit}${n === 1 ? "" : "s"} ago`;

  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return plural(diffMin, "minute");

  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return plural(diffHr, "hour");

  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 30) return plural(diffDay, "day");

  const diffMo = Math.floor(diffDay / 30);
  if (diffMo < 12) return plural(diffMo, "month");

  return plural(Math.floor(diffMo / 12), "year");
}

/**
 * Returns TanStack Table column definitions for the admin users DataTable.
 * `enableSorting: false` on every column is intentional — the server
 * connection has a fixed (created_at DESC, id ASC) order baked into cursor
 * encoding (see `.claude/rules/pagination.md`). Enabling client-side sort
 * would re-order the visible page only, leaving page boundaries inconsistent
 * with the cursors.
 */
export function getUsersColumns(): ColumnDef<AdminUserRow>[] {
  return [
    // Column 1 — Avatar + Name
    {
      id: "name",
      header: ({ column }) => <DataTableColumnHeader column={column} title="Name" />,
      enableSorting: false,
      cell: ({ row }) => {
        const { displayName, avatarUrl } = row.original;
        const initials = getInitials(displayName);

        return (
          <div className="flex items-center gap-3">
            <Avatar>
              {avatarUrl !== null ? (
                <AvatarImage src={avatarUrl} alt={displayName ?? "User avatar"} />
              ) : null}
              <AvatarFallback>{initials}</AvatarFallback>
            </Avatar>
            <span className="text-sm">
              {displayName !== null ? (
                displayName
              ) : (
                <span className="italic text-muted-foreground">No name</span>
              )}
            </span>
          </div>
        );
      },
    },

    // Column 2 — Roles
    {
      id: "roles",
      header: ({ column }) => <DataTableColumnHeader column={column} title="Roles" />,
      enableSorting: false,
      cell: ({ row }) => {
        const { roles } = row.original;

        if (roles.length === 0) {
          return <span className="text-muted-foreground text-sm">—</span>;
        }

        return (
          <div className="flex flex-wrap gap-1.5">
            {roles.map((role) => (
              <Badge key={role.id} variant={role.name === "admin" ? "admin" : "secondary"}>
                {role.name}
              </Badge>
            ))}
          </div>
        );
      },
    },

    // Column 3 — Last active
    {
      id: "lastActive",
      header: ({ column }) => <DataTableColumnHeader column={column} title="Last active" />,
      enableSorting: false,
      cell: ({ row }) => {
        return (
          <span className="text-sm text-muted-foreground">
            {formatRelativeTime(row.original.lastActive)}
          </span>
        );
      },
    },

    // Column 4 — Actions
    {
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => {
        const { id } = row.original;

        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label="Open user actions">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem asChild>
                <Link href={`/admin/users/${id}/edit`}>Edit</Link>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        );
      },
    },
  ];
}
