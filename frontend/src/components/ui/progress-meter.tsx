import { cn } from "@/lib/utils";

export type ProgressMeterProps = {
  value: number;
  max: number;
  label: string;
  className?: string;
};

/**
 * Presentation-only horizontal progress meter used by the learning-stats
 * per-deck acquisition rows. A coral fill (`bg-brand-primary`) advances over a
 * muted track (`bg-muted`); the width is derived once from `value / max` and
 * clamped to `[0, 100]` so a zero or negative `max` renders an empty bar rather
 * than dividing by zero or emitting `NaN`. The outer track carries the
 * `progressbar` a11y roles (`aria-label` / `aria-valuenow` / `aria-valuemin` /
 * `aria-valuemax`). It is a pure stateless component (no hooks), so it
 * intentionally omits `"use client"`.
 */
export function ProgressMeter({ value, max, label, className }: ProgressMeterProps) {
  const pct = max > 0 ? Math.min(100, Math.max(0, (value / max) * 100)) : 0;

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
