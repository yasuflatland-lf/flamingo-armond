import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the /catalog route. Mirrors the resolved grid layout
 * to prevent CLS while the GraphQL fetch streams in.
 */
export function CatalogSkeleton() {
  return (
    <ListingPageShell
      title={<Skeleton className="h-8 w-40" />}
      description={<Skeleton className="mt-2 h-4 w-80" />}
      toolbar={<Skeleton className="h-10 w-full max-w-sm" />}
    >
      <ul
        className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3"
        aria-busy="true"
        aria-label="Loading catalog"
        data-testid="catalog-skeleton"
      >
        {Array.from({ length: 6 }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder tiles have no stable id.
          <li key={i} className="flex flex-col gap-3 rounded-lg border border-border p-4">
            <Skeleton className="h-5 w-2/3" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-1/2" />
            <div className="mt-auto flex items-center justify-between">
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-8 w-24" />
            </div>
          </li>
        ))}
      </ul>
    </ListingPageShell>
  );
}
