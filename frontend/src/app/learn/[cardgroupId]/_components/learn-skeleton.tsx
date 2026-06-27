import { SWIPE_SESSION_GRID_CLASS } from "@/components/learn/swipe-session";
import { Skeleton } from "@/components/ui/skeleton";

// Shape mirrors the shared <SwipeSession> layout (banner row, card area, action
// bar) so the layout does not shift when streamed content replaces this
// placeholder. The outer grid class is imported from <SwipeSession> so the two
// can never drift. Used by both loading.tsx (route-segment fallback) and the
// in-page <Suspense>.
export function LearnSkeleton() {
  return (
    <section className={SWIPE_SESSION_GRID_CLASS} aria-busy="true" aria-label="Loading flashcards">
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
