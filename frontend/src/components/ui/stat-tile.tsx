import type { ComponentType } from "react";

export type StatTileProps = {
  label: string;
  value: string;
  caption?: string;
  icon?: ComponentType<{ className?: string }>;
};

/**
 * Presentation-only KPI card used by the `/stats` diagnostics row
 * (`DiagnosticsPanel`). It renders a single pre-formatted metric — a label, a
 * large numeric value, and an optional caption — inside shadcn card chrome.
 * Number formatting (locale, units, rounding) is the CALLER's responsibility;
 * `value` is a ready-to-render string and always carries `tabular-nums` so
 * columns of tiles align digit-for-digit. An optional `icon` component renders
 * muted in the header row.
 *
 * It is a pure stateless component (no hooks), so it intentionally omits
 * `"use client"`.
 */
export function StatTile({ label, value, caption, icon }: StatTileProps) {
  const Icon = icon;
  return (
    <div className="rounded-xl border bg-card text-card-foreground shadow-sm p-4">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">{label}</span>
        {Icon ? <Icon className="h-4 w-4 text-muted-foreground" /> : null}
      </div>
      <p className="mt-1 text-2xl font-bold tabular-nums">{value}</p>
      {caption ? <p className="text-xs text-muted-foreground">{caption}</p> : null}
    </div>
  );
}
