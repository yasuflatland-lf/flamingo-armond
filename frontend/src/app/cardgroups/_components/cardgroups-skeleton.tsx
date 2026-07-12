import { SkeletonRows } from "@/components/layout/list-skeleton";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the /cardgroups listing route. Mirrors the resolved
 * page layout to prevent CLS while the GraphQL fetch streams in.
 */
export function CardgroupsSkeleton() {
  return (
    <ListingPageShell
      title={<Skeleton className="h-8 w-48" />}
      primaryActions={<Skeleton className="hidden h-10 w-40 md:inline-flex" />}
      toolbar={<Skeleton className="h-10 w-full max-w-sm" />}
    >
      <SkeletonRows
        rowCount={5}
        ariaLabel="Loading cardgroups"
        testId="cardgroups-skeleton"
        rowClassName="rounded-lg border border-border p-4"
        renderRow={() => (
          <>
            <Skeleton className="h-5 w-2/3" />
            <Skeleton className="mt-2 h-4 w-1/3" />
          </>
        )}
      />
    </ListingPageShell>
  );
}
