import { Badge } from "@/components/ui/badge";
import { CefrLevel } from "@/generated/base-types";
import { cn } from "@/lib/utils";

/**
 * CefrBadge is the difficulty indicator for a Learn card.
 *
 * It is RESERVED for the card's top-right corner. Future FSRS badges
 * (#276 / #277) must claim a DIFFERENT corner so the two never collide.
 *
 * The badge is hidden entirely when `level` is null — the null contract
 * keeps callers dumb: they pass the (possibly absent) level straight
 * through and let this component decide whether anything renders.
 *
 * The element is purely presentational: a labelled <div>, never a control
 * (no tabIndex / onClick / role). Text always accompanies the band color so
 * color is never the sole signal.
 */

// Band buckets the five CEFR levels into three color families.
// Keep this an EXHAUSTIVE Record (not a switch with a default) so a future
// `C2` enum value becomes a one-line compile error here rather than silently
// falling into a default branch.
const bandOf: Record<CefrLevel, "a" | "b" | "c"> = {
  [CefrLevel.A1]: "a",
  [CefrLevel.A2]: "a",
  [CefrLevel.B1]: "b",
  [CefrLevel.B2]: "b",
  [CefrLevel.C1]: "c",
};

// Full literal class strings: Tailwind's scanner cannot see interpolated
// `bg-cefr-${band}` names, so the utilities must appear verbatim here.
const bandClass: Record<"a" | "b" | "c", string> = {
  a: "bg-cefr-a text-cefr-a-foreground",
  b: "bg-cefr-b text-cefr-b-foreground",
  c: "bg-cefr-c text-cefr-c-foreground",
};

interface CefrBadgeProps {
  /**
   * The card's CEFR level. The key is REQUIRED; the value is nullable —
   * `null` (or a stray `undefined`) renders nothing.
   */
  level: CefrLevel | null;
}

export function CefrBadge({ level }: CefrBadgeProps) {
  // `==` so a stray `undefined` is also handled defensively.
  if (level == null) {
    return null;
  }

  const band = bandOf[level];

  return (
    <Badge
      variant="outline"
      aria-label={`CEFR level ${level}`}
      className={cn(
        // This badge owns the card's top-right corner; it assumes a
        // `position: relative` ancestor, which the mount site provides.
        "absolute right-3 top-3 z-10",
        // Soft fill, no hard border, tightened horizontal padding.
        "border-transparent px-2",
        bandClass[band],
      )}
    >
      {level}
    </Badge>
  );
}
