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
  /** Status + count cluster rendered under the title. The page owns its md: reflow. */
  meta?: ReactNode;
  /** Trailing action cluster on the title row. The page owns its md: reflow. */
  actions?: ReactNode;
  /** Optional note rendered below meta, e.g. an empty-draft publish hint. */
  children?: ReactNode;
};

/**
 * The shared detail-page header layout: an inline back affordance flush to the
 * title (no standalone back row), with optional meta and trailing-action slots.
 * Used by the master-edit and cardgroup-detail headers. It owns layout only —
 * each consumer composes its own meta/actions, including responsive variants.
 */
export function DetailPageHeader({
  backHref,
  backLabel,
  title,
  meta,
  actions,
  children,
}: DetailPageHeaderProps) {
  return (
    <header className="mb-6">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <Link href={backHref} className="shrink-0 text-muted-foreground hover:text-foreground">
              <ChevronLeft aria-hidden="true" className="h-5 w-5" />
              <span className="sr-only">{backLabel}</span>
            </Link>
            <h1 className="min-w-0 truncate text-2xl font-semibold leading-tight">{title}</h1>
          </div>
          {meta ? <div className="mt-1">{meta}</div> : null}
          {children}
        </div>
        {actions ? <div className="flex shrink-0 items-center">{actions}</div> : null}
      </div>
    </header>
  );
}
