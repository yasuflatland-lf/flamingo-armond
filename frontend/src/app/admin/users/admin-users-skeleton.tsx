import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the /admin/users listing route. Mirrors the resolved
 * page layout to prevent CLS while the server component runs auth checks and
 * while the Apollo initial query resolves.
 */
export function AdminUsersSkeleton() {
  return (
    <main className="p-8">
      <div className="mb-6 flex items-center gap-4">
        <Skeleton className="h-8 w-24" />
        <Skeleton className="h-4 w-8" />
      </div>

      <div className="mb-6">
        <Skeleton className="h-10 w-full" />
      </div>

      <ul
        className="space-y-3"
        aria-busy="true"
        aria-label="Loading users"
        data-testid="admin-users-skeleton"
      >
        {Array.from({ length: 5 }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows have no stable id.
          <li key={i} className="rounded-md border border-border">
            <div className="flex flex-col gap-4 px-4 py-3 md:flex-row md:items-start">
              <div className="flex min-w-0 flex-1 items-start gap-4">
                <Skeleton className="h-10 w-10 shrink-0 rounded-full" />
                <div className="min-w-0 flex-1 space-y-1">
                  <Skeleton className="h-4 w-40" />
                  <Skeleton className="h-3 w-64" />
                </div>
              </div>
              <Skeleton className="h-8 w-16 self-start" />
            </div>
          </li>
        ))}
      </ul>
    </main>
  );
}
