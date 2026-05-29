import { Badge } from "@/components/ui/badge";
import type { CefrLevel } from "@/generated/graphql";
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
 * (no tabIndex / onClick). Text always accompanies the band color so color
 * is never the sole signal. It carries `role="img"` so the `aria-label` is
 * reliably announced: shadcn's `Badge` renders a bare <div> (implicit ARIA
 * role `generic`), and an `aria-label` on a name-prohibited `generic`
 * element is not reliably exposed to assistive tech.
 */

// Band buckets the five CEFR levels into three color families.
// Keep this an EXHAUSTIVE Record (not a switch with a default) so a future
// `C2` value becomes a one-line compile error here rather than silently
// falling into a default branch. `Record<CefrLevel, ...>` over the string
// union is just as exhaustive as over the runtime enum.
const bandOf: Record<CefrLevel, "a" | "b" | "c"> = {
  A1: "a",
  A2: "a",
  B1: "b",
  B2: "b",
  C1: "c",
  C2: "c",
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
   * The card's CEFR level. Required prop (no `?`): pass `null` explicitly
   * when the level is absent. `undefined` also renders nothing via the loose
   * `== null` guard.
   */
  level: CefrLevel | null;
}

export function CefrBadge({ level }: CefrBadgeProps) {
  // `==` so a stray `undefined` is also handled defensively.
  if (level == null) {
    return null;
  }

  // Index access on `bandOf` yields `| undefined` under `noUncheckedIndexedAccess`
  // (enabled in tsconfig), making the `| undefined` case unconditionally possible
  // at the type level. It also covers the runtime-lie case where a backend level
  // not yet in the generated union (deploy skew) reaches here. `bandOf` stays
  // typed as the exhaustive `Record<CefrLevel, ...>` so adding `C2` remains a
  // compile error.
  const band: "a" | "b" | "c" | undefined = bandOf[level];
  if (band === undefined) {
    // The union type can lie at runtime (a backend level not yet in the
    // generated enum, during deploy skew). Suppress the unstyled badge and
    // surface the gap to developers rather than rendering a colorless chip.
    console.warn(`[CefrBadge] Unrecognized CEFR level "${level}" — badge suppressed.`);
    return null;
  }

  return (
    <Badge
      variant="outline"
      role="img"
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
