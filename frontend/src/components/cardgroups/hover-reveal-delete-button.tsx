"use client";

import { Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export type HoverRevealDeleteButtonProps = {
  /** Fired when the trailing Delete control is activated. */
  onDelete: () => void;
  /** Accessible name for the Delete control. */
  ariaLabel: string;
  /** Disables the control (e.g. while a sibling mutation is in flight). */
  disabled?: boolean;
  /**
   * Additive row-specific layout overrides. The reveal + pointer-events
   * guard is wholly base-owned — callers must not re-pass guard tokens.
   */
  className?: string;
  /** Forwarded to the rendered `<button>` (e.g. `card-delete-${id}`). */
  "data-testid"?: string;
};

/**
 * Trailing Delete control shared by the swipe-to-delete list rows
 * (card / cardgroup / role). It is an `outline` icon button that stays
 * `opacity-0` until `sm:group-hover` / `sm:group-focus-within` on its row's
 * `group` wrapper, with a `motion-reduce:opacity-100` fallback so it is always
 * visible when the swipe layer is not rendered (the reduced-motion early-return
 * in `SwipeableRow`). It sits OUTSIDE the row link/edit-target so an outer click
 * does not consume the delete affordance.
 *
 * The reveal logic here is the load-bearing partner to `SwipeableRow`'s
 * reduced-motion early-return: when the swipe layer is absent the parent owns
 * the only delete affordance, so this button must reveal under
 * `prefers-reduced-motion`. Keep `motion-reduce:opacity-100` in the base.
 *
 * Every `opacity` arm carries a matching `pointer-events` arm ("visible iff
 * tappable"): `opacity-0` alone leaves the hidden button clickable, so below
 * the `sm` breakpoint a stray tap at the row's right edge would fire an
 * invisible delete. The guard lives in the base, never at call sites — a
 * consumer that omits it silently reintroduces the invisible-tap bug.
 */
export function HoverRevealDeleteButton({
  onDelete,
  ariaLabel,
  disabled,
  className,
  "data-testid": dataTestid,
}: HoverRevealDeleteButtonProps) {
  return (
    <Button
      type="button"
      variant="outline"
      size="icon"
      onClick={onDelete}
      disabled={disabled}
      aria-label={ariaLabel}
      data-testid={dataTestid}
      className={cn(
        "pointer-events-none opacity-0 sm:group-hover:pointer-events-auto sm:group-hover:opacity-100 sm:group-focus-within:pointer-events-auto sm:group-focus-within:opacity-100 motion-reduce:pointer-events-auto motion-reduce:opacity-100 transition-opacity",
        className,
      )}
    >
      <Trash2 aria-hidden="true" className="h-4 w-4" />
    </Button>
  );
}
