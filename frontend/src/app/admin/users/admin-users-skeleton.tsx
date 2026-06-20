import { ListSkeleton } from "@/components/layout/list-skeleton";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the /admin/users listing route. Mirrors the resolved
 * page layout to prevent CLS while the server component runs auth checks and
 * while the Apollo initial query resolves.
 */
export function AdminUsersSkeleton() {
  return (
    <ListSkeleton
      ariaLabel="Loading users"
      testId="admin-users-skeleton"
      renderRow={() => (
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
      )}
    />
  );
}
