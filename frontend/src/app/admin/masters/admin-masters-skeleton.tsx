import { ListSkeleton } from "@/components/layout/list-skeleton";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the /admin/masters listing route. Mirrors the
 * resolved layout to prevent CLS during the auth check + initial Apollo query.
 */
export function AdminMastersSkeleton() {
  return (
    <ListSkeleton
      ariaLabel="Loading masters"
      testId="admin-masters-skeleton"
      renderRow={() => (
        <div className="flex items-center gap-4 px-4 py-3">
          <Skeleton className="h-5 w-16 rounded-full" />
          <div className="min-w-0 flex-1 space-y-1">
            <Skeleton className="h-4 w-48" />
          </div>
          <Skeleton className="h-8 w-20" />
          <Skeleton className="h-8 w-16" />
        </div>
      )}
    />
  );
}
