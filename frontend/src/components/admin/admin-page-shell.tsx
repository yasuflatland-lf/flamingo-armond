import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface AdminPageShellProps {
  /** Page title shown as the leading heading. */
  title: ReactNode;
  /** Optional supporting copy under the title. */
  description?: ReactNode;
  /** Optional CTA cluster aligned to the trailing edge of the header row. */
  primaryActions?: ReactNode;
  /** Optional toolbar slot rendered between the header row and the content. */
  toolbar?: ReactNode;
  /** Main content (table, list, grid). */
  children: ReactNode;
  /** Optional class merge for the outer <main>. */
  className?: string;
}

/**
 * Shared shell for admin and admin-adjacent listing pages.
 *
 * Mirrors the shadcn-admin Tasks/Users `<Main>` shape: a flex column with
 * a wrap-friendly title row, an optional toolbar slot, and the page body.
 * Padding matches the existing `p-8` adopted across cardgroups/admin pages
 * after the recent max-width removal, so this shell stays liquid.
 *
 * Intentionally unaware of authorization — admin gating lives in
 * `app/admin/layout.tsx`. AdminPageShell is reusable from any listing page
 * (admin or not) that wants the same header + toolbar geometry.
 */
export function AdminPageShell({
  title,
  description,
  primaryActions,
  toolbar,
  children,
  className,
}: AdminPageShellProps) {
  return (
    <main className={cn("flex flex-1 flex-col gap-4 p-8 sm:gap-6", className)}>
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
          {description != null && <p className="text-muted-foreground">{description}</p>}
        </div>
        {primaryActions != null && <div className="flex items-center gap-2">{primaryActions}</div>}
      </div>
      {toolbar != null && <div data-slot="toolbar">{toolbar}</div>}
      {children}
    </main>
  );
}
