import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface ListingPageShellProps {
  /** Page title. Centered on mobile, left-aligned on desktop. */
  title: ReactNode;
  /** Optional total count rendered as a neutral pill: below the title on mobile, inline to its right on desktop. */
  count?: number;
  /** Optional fully-formatted, localized label rendered inside the count pill (e.g. "14 total"). Falls back to the bare `count` number when omitted. */
  countLabel?: ReactNode;
  /** Optional supporting copy under the title. */
  description?: ReactNode;
  /** Optional CTA cluster (desktop create button etc.): below the title on mobile, on the trailing edge of the header row on desktop. */
  primaryActions?: ReactNode;
  /** Optional toolbar slot rendered between the header row and the content. */
  toolbar?: ReactNode;
  /** Main content (table, list, grid). */
  children: ReactNode;
  /** Optional class merge for the outer <main>. */
  className?: string;
}

/**
 * Shared shell for listing pages (admin, cardgroups, or any similar page).
 *
 * The header is responsive:
 *  - Mobile (`< md`): a centered column — title, then the `(N)` count under it.
 *    The create button is hidden here (mobile creates via the top-bar `+`).
 *  - Desktop (`>= md`): a `justify-between` row — title with the `(N)` count
 *    inline to its right on the leading edge, `primaryActions` on the trailing
 *    edge. Mirrors the shadcn-admin Tasks/Users `<Main>` header shape.
 *
 * The optional toolbar slot (search/filter input) and the page body sit below
 * the header in both layouts. Padding matches the existing `p-8` adopted across
 * listing pages after the max-width removal, so this shell stays liquid.
 *
 * Intentionally unaware of authorization — admin gating lives in
 * `app/admin/layout.tsx`. ListingPageShell is reusable from any listing page
 * (admin or not) that wants the same header + toolbar geometry.
 */
export function ListingPageShell({
  title,
  count,
  countLabel,
  description,
  primaryActions,
  toolbar,
  children,
  className,
}: ListingPageShellProps) {
  return (
    <main className={cn("flex flex-1 flex-col gap-4 p-8 sm:gap-6", className)}>
      <div
        data-slot="page-header"
        className="flex flex-col items-center gap-2 text-center md:flex-row md:items-center md:justify-between md:gap-4 md:text-left"
      >
        <div className="flex flex-col items-center gap-1 md:items-start">
          <div className="flex flex-col items-center gap-1 md:flex-row md:items-baseline md:gap-2">
            <h1 className="text-page-title font-bold leading-tight tracking-normal">{title}</h1>
            {count != null && (
              <span className="inline-flex items-center rounded-full border bg-muted px-2.5 py-0.5 text-sm font-medium text-muted-foreground tabular-nums">
                {countLabel ?? count}
              </span>
            )}
          </div>
          {description != null && <div className="text-muted-foreground">{description}</div>}
        </div>
        {primaryActions != null && (
          <div className="flex items-center justify-center gap-2 md:justify-end">
            {primaryActions}
          </div>
        )}
      </div>
      {toolbar != null && <div data-slot="toolbar">{toolbar}</div>}
      {children}
    </main>
  );
}
