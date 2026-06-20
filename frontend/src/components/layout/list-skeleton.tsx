import type { ReactNode } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

interface ListSkeletonProps {
  /** Number of placeholder rows to render. Defaults to 5. */
  rowCount?: number;
  /** Accessible name for the busy list (e.g. "Loading users"). */
  ariaLabel: string;
  /** `data-testid` for the busy list (e.g. "admin-users-skeleton"). */
  testId: string;
  /** Renders the per-row body. Called once per placeholder row. */
  renderRow: () => ReactNode;
  /**
   * Optional override for the header above the list. Defaults to the standard
   * listing-page header: a title row of two Skeletons plus a search-input
   * Skeleton. Pass a custom node when the page's resolved header differs
   * (e.g. the roles list has no search input).
   */
  header?: ReactNode;
  /** Optional class merge for the outer `<main>`. */
  className?: string;
  /** Optional class merge for each placeholder `<li>`. */
  rowClassName?: string;
}

/**
 * Shared loading placeholder for listing routes. Owns the `<main>` landmark,
 * the header (title + search Skeletons by default), and the `Array.from` row
 * loop. Mirrors the resolved page layout to prevent CLS while the server
 * component runs auth checks and the initial Apollo query resolves.
 *
 * The `noArrayIndexKey` biome-ignore for the static placeholder rows lives here
 * once, so individual list skeletons no longer repeat it.
 */
export function ListSkeleton({
  rowCount = 5,
  ariaLabel,
  testId,
  renderRow,
  header,
  className,
  rowClassName,
}: ListSkeletonProps) {
  return (
    <main className={cn("p-8", className)}>
      {header ?? (
        <>
          <div className="mb-6 flex items-center gap-4">
            <Skeleton className="h-8 w-24" />
            <Skeleton className="h-4 w-8" />
          </div>

          <div className="mb-6">
            <Skeleton className="h-10 w-full" />
          </div>
        </>
      )}

      <ul className="space-y-3" aria-busy="true" aria-label={ariaLabel} data-testid={testId}>
        {Array.from({ length: rowCount }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows have no stable id.
          <li key={i} className={cn("rounded-md border border-border", rowClassName)}>
            {renderRow()}
          </li>
        ))}
      </ul>
    </main>
  );
}
