"use client";

import { useTranslations } from "next-intl";
import { HoverRevealDeleteButton } from "@/components/cardgroups/hover-reveal-delete-button";
import { SwipeableRow } from "@/components/cardgroups/swipeable-row";

export type RoleListItemProps = {
  id: string;
  name: string;
  /** When true, the row is non-clickable and the trailing delete button is disabled. */
  isSystem: boolean;
  /** Disables the delete button while a delete (or any sibling) mutation is in flight. */
  busy: boolean;
  onEdit: (id: string) => void;
  onDelete: (id: string) => void;
};

/**
 * One row in the roles list. Editable roles wrap their content in a
 * SwipeableRow so mobile users can swipe-to-delete. System roles render as
 * plain content with reduced opacity to telegraph the non-clickable state
 * and skip the hover background — no swipe gesture, no delete button.
 *
 * The trailing delete button sits outside the link so an outer click does
 * not consume the delete affordance — it is the shared
 * HoverRevealDeleteButton, the same control used by cardgroup-list-item.tsx
 * and card-row.tsx.
 */
export function RoleListItem({ id, name, isSystem, busy, onEdit, onDelete }: RoleListItemProps) {
  const t = useTranslations("Admin");
  if (isSystem) {
    return (
      <li
        className="flex items-center gap-2 rounded-lg border border-border pr-2 opacity-60"
        data-testid={`admin-role-row-${id}`}
      >
        <div className="flex min-w-0 flex-1 flex-col gap-1 p-4">
          <span className="truncate font-medium text-foreground" data-testid="admin-role-name">
            {name}
          </span>
          <span className="text-xs text-muted-foreground">{t("systemRole")}</span>
        </div>
      </li>
    );
  }

  return (
    <SwipeableRow onDelete={() => onDelete(id)} disabled={busy}>
      <li
        className="group flex items-center gap-2 rounded-lg border border-border pr-2 hover:bg-accent active:bg-accent transition-colors"
        data-testid={`admin-role-row-${id}`}
      >
        <button
          type="button"
          onClick={() => onEdit(id)}
          aria-label={`Edit role ${name}`}
          className="flex min-w-0 flex-1 flex-col gap-1 p-4 text-left"
        >
          <span className="truncate font-medium text-foreground" data-testid="admin-role-name">
            {name}
          </span>
        </button>
        <HoverRevealDeleteButton
          onDelete={() => onDelete(id)}
          disabled={busy}
          ariaLabel={`Delete ${name}`}
          data-testid={`admin-role-delete-btn-${id}`}
        />
      </li>
    </SwipeableRow>
  );
}
