import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the /catalog route. Mirrors the resolved single-column
 * list layout (CatalogListItem rows — name + card-count stat over a badge row)
 * to prevent CLS while the GraphQL fetch streams in.
 */
export function CatalogSkeleton() {
  return (
    <ListingPageShell
      title={<Skeleton className="h-8 w-40" />}
      toolbar={<Skeleton className="h-10 w-full max-w-sm" />}
    >
      <ul
        className="space-y-2"
        aria-busy="true"
        aria-label="Loading catalog"
        data-testid="catalog-skeleton"
      >
        {Array.from({ length: 6 }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows have no stable id.
          <li key={i} className="rounded-md border border-border p-4 sm:py-3">
            <div className="flex items-baseline gap-3">
              <Skeleton className="h-5 min-w-0 flex-1" />
              <Skeleton className="h-5 w-14 shrink-0" />
            </div>
            <div className="mt-1.5 flex items-center gap-2">
              <Skeleton className="h-4 w-32" />
            </div>
          </li>
        ))}
      </ul>
    </ListingPageShell>
  );
}
