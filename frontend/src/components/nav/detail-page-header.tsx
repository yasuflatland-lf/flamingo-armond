"use client";

import { ChevronLeft } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

type DetailPageHeaderProps = {
  /** Destination of the inline back affordance. */
  backHref: string;
  /** Accessible (sr-only) label for the back link, e.g. t("backToList"). */
  backLabel: string;
  title: string;
  /** Count cluster centered in the app-bar row, between the back affordance and
   *  the trailing actions. The page owns its md: reflow. */
  meta?: ReactNode;
  /** Trailing action cluster on the app-bar row (right). The page owns its md: reflow. */
  actions?: ReactNode;
  /** Optional status indicator (e.g. a publish-state chip/badge) centered
   *  directly below the title, reading as an attribute of the title. Sits one
   *  tier above `children`. The page owns its md: reflow. */
  status?: ReactNode;
  /** Optional note rendered below the status slot, e.g. an empty-draft publish hint. */
  children?: ReactNode;
};

/**
 * The shared detail-page header layout: an app-bar row carrying an inline back
 * affordance (left), a centered count meta cluster (center), and a trailing
 * action cluster (right); the page title sits centered on its own row one tier
 * below, with an optional status indicator centered directly beneath the title.
 * The 1fr/auto/1fr app-bar grid keeps the meta cluster centered regardless of
 * the back/action cluster widths. Used by the master-edit header. It owns layout
 * only — the consumer composes its own meta/actions/status, including responsive
 * variants.
 */
export function DetailPageHeader({
  backHref,
  backLabel,
  title,
  meta,
  actions,
  status,
  children,
}: DetailPageHeaderProps) {
  return (
    <header className="mb-6">
      {/* App-bar row — inline back (left), centered meta (center), actions
          (right). The 1fr/auto/1fr grid keeps the meta cluster truly centered
          regardless of the back/action cluster widths. */}
      <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
        <Link
          href={backHref}
          className="shrink-0 justify-self-start rounded-sm text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <ChevronLeft aria-hidden="true" className="h-5 w-5" />
          <span className="sr-only">{backLabel}</span>
        </Link>
        <div className="justify-self-center">{meta}</div>
        <div className="flex items-center justify-self-end">{actions}</div>
      </div>
      {/* Title row — centered. A wider gap above (mt-2) than the title→status gap
          below (mt-1) sets the title+status pair apart from the app-bar so the
          title heads its own group rather than binding to the utility row. */}
      <div className="mt-2 flex items-center justify-center">
        <h1 className="min-w-0 truncate text-2xl font-semibold leading-tight">{title}</h1>
      </div>
      {/* Status row — centered directly below the title with a tight mt-1
          (title→status 4px < app-bar→title 8px) so it binds to the title and
          reads as an attribute of it. */}
      {status ? <div className="mt-1 flex items-center justify-center">{status}</div> : null}
      {children}
    </header>
  );
}
