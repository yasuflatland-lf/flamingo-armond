import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading-state placeholder for the `/cards/new` route. Shape mirrors the
 * `<CardsNewClient>` layout — a cardgroup chip row, two labeled input rows
 * (Front / Back), and a submit + cancel button row — so the page does not
 * shift when the streamed form arrives.
 *
 * Used by both `loading.tsx` (route-segment navigation fallback) and the
 * in-page `<Suspense>` boundary that wraps the bootstrap GraphQL fetch.
 */
export function CardsNewSkeleton() {
  return (
    <section className="space-y-6" aria-busy="true" aria-label="Loading new card form">
      <div className="flex items-center gap-3">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-8 w-40" />
      </div>

      <div className="space-y-3">
        <div className="space-y-1">
          <Skeleton className="h-4 w-12" />
          <Skeleton className="h-10 w-full" />
        </div>
        <div className="space-y-1">
          <Skeleton className="h-4 w-12" />
          <Skeleton className="h-10 w-full" />
        </div>
        <div className="flex items-center gap-2">
          <Skeleton className="h-10 w-24" />
          <Skeleton className="h-10 w-20" />
        </div>
      </div>
    </section>
  );
}
