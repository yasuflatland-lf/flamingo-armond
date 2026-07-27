import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for `/stats`. Mirrors the resolved page header and card
 * grid so the streamed statistics do not shift the surrounding layout.
 */
export function StatsSkeleton() {
  return (
    <main
      className="flex flex-1 flex-col gap-4 p-8 sm:gap-6"
      aria-busy="true"
      aria-label="Loading learning statistics"
      data-testid="stats-skeleton"
    >
      <div
        data-slot="page-header"
        className="flex flex-col items-center gap-2 text-center md:flex-row md:items-center md:justify-between md:gap-4 md:text-left"
      >
        <div className="flex flex-col items-center gap-1 md:items-start">
          <Skeleton className="h-8 w-32" />
          <Skeleton className="h-5 w-64 max-w-full" />
        </div>
      </div>

      <div className="flex flex-col gap-4 sm:gap-6">
        <Skeleton className="h-32 w-full rounded-xl" />
        <Skeleton className="h-44 w-full rounded-xl" />
        <div className="grid gap-4 lg:grid-cols-[4fr_3fr]">
          <Skeleton className="h-72 w-full rounded-xl" />
          <Skeleton className="h-72 w-full rounded-xl" />
        </div>
      </div>
    </main>
  );
}
