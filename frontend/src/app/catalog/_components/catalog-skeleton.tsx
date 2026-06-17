import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the /catalog route. Mirrors the resolved single-column
 * list layout (CatalogListItem rows — two-tier on mobile, single inline row on
 * sm+) to prevent CLS while the GraphQL fetch streams in.
 */
export function CatalogSkeleton() {
  return (
    <ListingPageShell
      title={<Skeleton className="h-8 w-40" />}
      description={<Skeleton className="mt-2 h-4 w-80" />}
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
          <li key={i} className="rounded-md border border-border">
            <div className="flex flex-wrap items-center gap-x-3 gap-y-3 p-4 sm:py-3">
              <Skeleton className="h-5 w-full sm:w-auto sm:flex-1" />
              <Skeleton className="h-5 w-12 shrink-0" />
              <Skeleton className="h-4 w-16 shrink-0" />
              <Skeleton className="ml-auto h-9 w-24 shrink-0" />
            </div>
          </li>
        ))}
      </ul>
    </ListingPageShell>
  );
}
