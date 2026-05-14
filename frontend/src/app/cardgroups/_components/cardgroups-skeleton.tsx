import { ListingPageShell } from "@/components/layout/listing-page-shell";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading-state placeholder for the /cardgroups listing route. Shape mirrors
 * the `<CardgroupsClient>` layout — header (via ListingPageShell), toolbar
 * row, and a vertical stack of cardgroup row rectangles — so the page layout
 * does not shift when the streamed content arrives (CLS minimization).
 *
 * Used by both `loading.tsx` (route-segment navigation fallback) and the
 * in-page `<Suspense>` boundary that wraps the GraphQL fetch.
 *
 * Renders inside the same `ListingPageShell` the resolved page uses, so the
 * header / toolbar geometry is identical between the loading and resolved
 * states. The `primaryActions` and `toolbar` slots are filled with skeleton
 * primitives sized to the real CTA button and search input respectively.
 */
export function CardgroupsSkeleton() {
  return (
    <ListingPageShell
      title={<Skeleton className="h-8 w-48" />}
      description={<Skeleton className="mt-2 h-4 w-72" />}
      primaryActions={<Skeleton className="hidden h-10 w-40 md:inline-flex" />}
      toolbar={<Skeleton className="h-10 w-full max-w-sm" />}
    >
      <ul
        className="space-y-3"
        aria-busy="true"
        aria-label="Loading cardgroups"
        data-testid="cardgroups-skeleton"
      >
        {Array.from({ length: 5 }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows have no stable id.
          <li key={i} className="rounded-lg border border-border p-4">
            <Skeleton className="h-5 w-2/3" />
            <Skeleton className="mt-2 h-4 w-1/3" />
          </li>
        ))}
      </ul>
    </ListingPageShell>
  );
}
