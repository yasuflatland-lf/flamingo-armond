import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the /admin/masters listing route. Mirrors the
 * resolved layout to prevent CLS during the auth check + initial Apollo query.
 */
export function AdminMastersSkeleton() {
  return (
    <main className="p-8">
      <div className="mb-6 flex items-center gap-4">
        <Skeleton className="h-8 w-28" />
        <Skeleton className="h-4 w-8" />
      </div>

      <div className="mb-6">
        <Skeleton className="h-10 w-full" />
      </div>

      <ul
        className="space-y-3"
        aria-busy="true"
        aria-label="Loading masters"
        data-testid="admin-masters-skeleton"
      >
        {Array.from({ length: 5 }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows have no stable id.
          <li key={i} className="rounded-md border border-border">
            <div className="flex items-center gap-4 px-4 py-3">
              <Skeleton className="h-5 w-16 rounded-full" />
              <div className="min-w-0 flex-1 space-y-1">
                <Skeleton className="h-4 w-48" />
              </div>
              <Skeleton className="h-8 w-20" />
              <Skeleton className="h-8 w-16" />
            </div>
          </li>
        ))}
      </ul>
    </main>
  );
}
