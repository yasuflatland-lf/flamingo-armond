"use client";

import { Trash2 } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";

export type RoleListItemProps = {
  id: string;
  name: string;
  /** When true, the row is non-clickable and the trailing delete button is disabled. */
  isSystem: boolean;
  /** Disables the delete button while a delete (or any sibling) mutation is in flight. */
  busy: boolean;
  onDelete: (id: string) => void;
};

/**
 * One row in the roles list. Editable roles wrap their name area in a
 * <Link> that fills the row (cursor-pointer + focus ring inherited from
 * the anchor). System roles render as plain content with reduced opacity
 * to telegraph the non-clickable state and skip the hover background.
 *
 * The trailing delete button sits outside the link so an outer click does
 * not consume the delete affordance — same pattern as
 * frontend/src/components/cardgroups/cardgroup-list-item.tsx.
 */
export function RoleListItem({ id, name, isSystem, busy, onDelete }: RoleListItemProps) {
  const wrapperClasses = isSystem
    ? "flex items-center gap-2 rounded-lg border border-border pr-2 opacity-60"
    : "flex items-center gap-2 rounded-lg border border-border pr-2 hover:bg-accent transition-colors";

  return (
    <li className={wrapperClasses} data-testid={`admin-role-row-${id}`}>
      {isSystem ? (
        <div className="flex min-w-0 flex-1 flex-col gap-1 p-4">
          <span className="truncate font-medium text-foreground" data-testid="admin-role-name">
            {name}
          </span>
          <span className="text-xs text-muted-foreground">System role</span>
        </div>
      ) : (
        <Link href={`/admin/roles/${id}/edit`} className="flex min-w-0 flex-1 flex-col gap-1 p-4">
          <span className="truncate font-medium text-foreground" data-testid="admin-role-name">
            {name}
          </span>
        </Link>
      )}
      <Button
        type="button"
        variant="outline"
        size="icon"
        onClick={() => onDelete(id)}
        disabled={busy || isSystem}
        aria-label={`Delete ${name}`}
        data-testid={`admin-role-delete-btn-${id}`}
      >
        <Trash2 aria-hidden="true" className="h-4 w-4" />
      </Button>
    </li>
  );
}
