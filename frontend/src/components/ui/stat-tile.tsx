import type { ReactNode } from "react";

export type StatTileProps = {
  label: string;
  value: string;
  caption?: string;
  action?: ReactNode;
};

/**
 * Presentation-only KPI card used by the `/stats` diagnostics row
 * (`DiagnosticsPanel`). It renders a single pre-formatted metric — a label, a
 * large numeric value, and an optional caption — inside shadcn card chrome.
 * Number formatting (locale, units, rounding) is the CALLER's responsibility;
 * `value` is a ready-to-render string and always carries `tabular-nums` so
 * columns of tiles align digit-for-digit. An optional `action` node renders in
 * the top-right header slot (e.g. a help-popover trigger).
 *
 * This shell stays presentational and intentionally omits `"use client"`; the
 * `action` node may itself be an interactive client component.
 */
export function StatTile({ label, value, caption, action }: StatTileProps) {
  return (
    <div className="rounded-xl border bg-card text-card-foreground shadow-sm p-4">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">{label}</span>
        {action}
      </div>
      <p className="mt-1 text-2xl font-bold tabular-nums">{value}</p>
      {caption ? <p className="text-xs text-muted-foreground">{caption}</p> : null}
    </div>
  );
}
