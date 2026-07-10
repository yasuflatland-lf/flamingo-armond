import { cn } from "@/lib/utils";

export type ProgressMeterProps = {
  value: number;
  max: number;
  label: string;
  className?: string;
};

/**
 * Presentation-only horizontal progress meter used by the `/stats` per-deck
 * acquisition rows (`PerDeckList`). A coral fill (`bg-brand-primary`) advances over
 * a muted track (`bg-muted`). The rendered percentage is derived once from
 * `value / max` and defensively clamped to `[0, 100]`: a non-positive `max`
 * (division by zero) and a non-finite or negative `value` all collapse to an
 * empty bar, so `aria-valuenow` and the fill width are always a valid
 * percentage — never `NaN`, which the CSSOM would silently drop and leave the
 * block-level fill at full container width (a misleading "100%" bar). The outer
 * track carries the `progressbar` a11y roles (`aria-label` / `aria-valuenow` /
 * `aria-valuemin` / `aria-valuemax`); the optional `className` merges onto that
 * track so callers can adjust height or spacing. It is a pure stateless
 * component (no hooks), so it intentionally omits `"use client"`.
 */
export function ProgressMeter({ value, max, label, className }: ProgressMeterProps) {
  const safeValue = Number.isFinite(value) ? value : 0;
  const pct = max > 0 ? Math.min(100, Math.max(0, (safeValue / max) * 100)) : 0;

  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuenow={Math.round(pct)}
      aria-valuemin={0}
      aria-valuemax={100}
      className={cn("h-2 w-full overflow-hidden rounded-full bg-muted", className)}
    >
      <div className="h-full rounded-full bg-brand-primary" style={{ width: `${pct}%` }} />
    </div>
  );
}
