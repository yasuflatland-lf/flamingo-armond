"use client";

import { ChevronDown } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

type SplitButtonMenuItem = {
  /** Stable React key for the item. */
  key: string;
  /** Leading icon, e.g. `<Plus className="h-4 w-4" />`. */
  icon?: ReactNode;
  label: ReactNode;
  onSelect: () => void;
  /** Renders the item in the destructive (deep-red) intent. */
  destructive?: boolean;
  disabled?: boolean;
  "data-testid"?: string;
};

type Props = {
  items: SplitButtonMenuItem[];
  /** Accessible label for the chevron trigger (icon-only). */
  triggerLabel: string;
  /**
   * Variant/size MUST match the sibling primary button so the two flush
   * segments read as one control.
   */
  variant?: ComponentProps<typeof Button>["variant"];
  size?: ComponentProps<typeof Button>["size"];
  /** test id for the chevron trigger button. */
  "data-testid"?: string;
};

/**
 * The trailing chevron segment of a split button: an icon-only trigger that
 * opens a dropdown of secondary actions. The primary action stays at the call
 * site (rendered flush to the left with `rounded-r-none`) so each consumer keeps
 * full control over its primary's state (variant, disabled, loading, link vs
 * button). Used by the cardgroup/master deck-detail action clusters.
 */
export function SplitButtonMenu({
  items,
  triggerLabel,
  variant = "outline",
  size = "sm",
  "data-testid": testId,
}: Props) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant={variant}
          size={size}
          className="rounded-l-none border-l px-2"
          aria-label={triggerLabel}
          data-testid={testId}
        >
          <ChevronDown aria-hidden="true" className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {items.map((item) => (
          <DropdownMenuItem
            key={item.key}
            onSelect={item.onSelect}
            disabled={item.disabled}
            data-testid={item["data-testid"]}
            className={cn("gap-2", item.destructive && "text-destructive focus:text-destructive")}
          >
            {item.icon}
            {item.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
