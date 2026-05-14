import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading-state placeholder for the learn route. Shape mirrors the
 * `<LearnClient>` layout — an optional banner row, a single card area, and an
 * action bar — so the layout does not shift when streamed content arrives.
 *
 * Used by both `loading.tsx` (route-segment navigation fallback) and the
 * in-page `<Suspense>` boundary that wraps the GraphQL `Promise.all`.
 */
export function LearnSkeleton() {
  return (
    <section
      className="grid min-h-0 flex-1 grid-rows-[auto_1fr_auto] gap-3"
      aria-busy="true"
      aria-label="Loading flashcards"
    >
      {/* Placeholder for the optional error banner row, matching the empty
          spacer LearnClient renders when no banner is present. */}
      <div aria-hidden="true" />

      {/* Card area — single rectangle approximating the swipeable card stack. */}
      <div className="relative flex min-h-0 items-center justify-center overflow-hidden">
        <Skeleton className="aspect-[3/4] w-full max-w-md" />
      </div>

      {/* Action bar — three rate buttons plus a leading label area. */}
      <div className="flex items-center justify-center gap-3 py-2">
        <Skeleton className="h-12 w-20" />
        <Skeleton className="h-12 w-20" />
        <Skeleton className="h-12 w-20" />
      </div>
    </section>
  );
}
