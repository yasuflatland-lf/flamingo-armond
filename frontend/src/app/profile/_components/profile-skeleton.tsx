import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for `/profile`. Mirrors the resolved profile summary and
 * settings form so the streamed fields keep their final positions.
 */
export function ProfileSkeleton() {
  return (
    <main
      className="p-8"
      aria-busy="true"
      aria-label="Loading profile"
      data-testid="profile-skeleton"
    >
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-8">
        <section className="space-y-3 border-b pb-6">
          <div className="flex items-center gap-1">
            <Skeleton className="h-8 w-32" />
            <Skeleton className="size-8" />
          </div>
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="space-y-1">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-4 w-32" />
            </div>
            <div className="space-y-1">
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-4 w-40 max-w-full" />
            </div>
            <div className="space-y-1">
              <Skeleton className="h-4 w-12" />
              <Skeleton className="h-4 w-36" />
            </div>
          </div>
        </section>

        <section className="space-y-4">
          <Skeleton className="h-7 w-28" />
          <div className="space-y-2">
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-10 w-full sm:w-56" />
          </div>
          <div className="space-y-2">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-10 w-full max-w-xs" />
          </div>
        </section>

        <section className="space-y-4 border-t border-destructive/30 pt-6">
          <Skeleton className="h-7 w-32" />
          <Skeleton className="h-10 w-40" />
        </section>
      </div>
    </main>
  );
}
