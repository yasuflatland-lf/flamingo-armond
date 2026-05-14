import { Skeleton } from "@/components/ui/skeleton";

// Shape mirrors the <LearnClient> layout (banner row, card area, action bar) so
// the layout does not shift when streamed content replaces this placeholder.
// Used by both loading.tsx (route-segment fallback) and the in-page <Suspense>.
export function LearnSkeleton() {
  return (
    <section
      className="grid min-h-0 flex-1 grid-rows-[auto_1fr_auto] gap-3"
      aria-busy="true"
      aria-label="Loading flashcards"
    >
      {/* Banner row — empty spacer matching LearnClient's no-banner state. */}
      <div aria-hidden="true" />

      {/* Card area — approximates the swipeable card stack. */}
      <div className="relative flex min-h-0 items-center justify-center overflow-hidden">
        <Skeleton className="aspect-[3/4] w-full max-w-md" />
      </div>

      {/* Action bar — three rate buttons. */}
      <div className="flex items-center justify-center gap-3 py-2">
        <Skeleton className="h-12 w-20" />
        <Skeleton className="h-12 w-20" />
        <Skeleton className="h-12 w-20" />
      </div>
    </section>
  );
}
