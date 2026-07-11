"use client";

import { CircleHelp } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

/**
 * Icon-only help affordance for a `/stats` diagnostic tile. `CircleHelp` is the
 * tap/click trigger (labelled by `ariaLabel`, since the glyph is decorative);
 * the popover explains the metric in plain language. Copy-agnostic — the caller
 * resolves catalog strings and passes ready `title` / `body` / `ariaLabel`, so
 * this component never touches `useTranslations`.
 */
export function StatHint({
  title,
  body,
  ariaLabel,
}: {
  title: string;
  body: string;
  ariaLabel: string;
}) {
  return (
    <Popover>
      <PopoverTrigger
        aria-label={ariaLabel}
        className="text-muted-foreground transition-colors hover:text-foreground focus-visible:text-foreground focus-visible:outline-none"
      >
        <CircleHelp aria-hidden="true" className="h-4 w-4" />
      </PopoverTrigger>
      <PopoverContent side="top">
        <p className="text-sm font-semibold">{title}</p>
        <p className="mt-1 text-xs text-muted-foreground">{body}</p>
      </PopoverContent>
    </Popover>
  );
}
