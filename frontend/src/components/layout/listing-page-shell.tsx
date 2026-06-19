import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface ListingPageShellProps {
  /** Page title shown as the centered heading. */
  title: ReactNode;
  /** Optional total-count badge rendered as a muted `(N)` subtitle under the title. */
  count?: number;
  /** Optional supporting copy under the title. */
  description?: ReactNode;
  /** Optional CTA cluster centered below the title (desktop create button etc.). */
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
 * Mirrors the shadcn-admin Tasks/Users `<Main>` shape: a flex column with
 * a wrap-friendly title row, an optional toolbar slot, and the page body.
 * Padding matches the existing `p-8` adopted across listing pages
 * after the recent max-width removal, so this shell stays liquid.
 *
 * Intentionally unaware of authorization — admin gating lives in
 * `app/admin/layout.tsx`. ListingPageShell is reusable from any listing page
 * (admin or not) that wants the same header + toolbar geometry.
 */
export function ListingPageShell({
  title,
  count,
  description,
  primaryActions,
  toolbar,
  children,
  className,
}: ListingPageShellProps) {
  return (
    <main className={cn("flex flex-1 flex-col gap-4 p-8 sm:gap-6", className)}>
      <div className="flex flex-col items-center gap-2 text-center">
        <h1 className="text-page-title font-bold leading-tight tracking-normal">{title}</h1>
        {count != null && <p className="text-sm text-muted-foreground">({count})</p>}
        {description != null && <div className="text-muted-foreground">{description}</div>}
        {primaryActions != null && (
          <div className="flex items-center justify-center gap-2">{primaryActions}</div>
        )}
      </div>
      {toolbar != null && <div data-slot="toolbar">{toolbar}</div>}
      {children}
    </main>
  );
}
