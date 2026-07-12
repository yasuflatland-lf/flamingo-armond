import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export type EmptyStateProps = {
  /** Optional muted icon rendered above the heading (e.g. a lucide icon). */
  icon?: ReactNode;
  /** Optional heading; omitted for body-only empty blocks. */
  heading?: ReactNode;
  /** Body copy — every empty state explains itself, so this is required. */
  body: ReactNode;
  /**
   * Optional CTA composition rendered below the body. The caller owns the inner
   * layout (single button, or a responsive `flex` row of links) so the emphasis
   * ladder can differ per state; the primitive only owns the top spacing.
   */
  actions?: ReactNode;
  /** `data-testid` on the container (preserves per-site test hooks). */
  testId?: string;
  /** Extra container classes: padding, rounding, max-width, or layout overrides. */
  className?: string;
  /** Extra heading classes (e.g. a smaller `text-sm` for a calm state). */
  headingClassName?: string;
  /** Extra body classes (e.g. `mx-auto max-w-md`, or a smaller `text-xs`). */
  bodyClassName?: string;
};

/**
 * Shared dashed-border empty block used across `/stats`, the card + catalog
 * deck lists, and the learn session-complete screen. It centralizes the
 * hand-rolled `rounded-lg border border-dashed border-border p-8 text-center`
 * look (icon / heading / body / actions slots) so the dashed empty-state visual
 * language lives in one place. Per-site divergences (compact `p-6` lists, a
 * width-capped section, a calm smaller struggling state) ride on the
 * `className` / `headingClassName` / `bodyClassName` escape hatches, which merge
 * over the defaults via `cn` (tailwind-merge, so the override wins).
 *
 * Presentation-only; it stays free of `"use client"` so it can render inside
 * either a server or a client tree. The `actions` node may itself be an
 * interactive client component.
 */
export function EmptyState({
  icon,
  heading,
  body,
  actions,
  testId,
  className,
  headingClassName,
  bodyClassName,
}: EmptyStateProps) {
  const hasAbove = icon != null || heading != null;
  return (
    <div
      data-testid={testId}
      className={cn(
        "flex flex-col items-center rounded-lg border border-dashed border-border p-8 text-center",
        className,
      )}
    >
      {icon}
      {heading != null ? (
        <h2 className={cn("mt-2 text-xl font-semibold", headingClassName)}>{heading}</h2>
      ) : null}
      <div className={cn(hasAbove && "mt-2", "text-sm text-muted-foreground", bodyClassName)}>
        {body}
      </div>
      {actions != null ? <div className="mt-6">{actions}</div> : null}
    </div>
  );
}
