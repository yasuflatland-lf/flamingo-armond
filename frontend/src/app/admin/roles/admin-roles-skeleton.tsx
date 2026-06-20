import { ListSkeleton } from "@/components/layout/list-skeleton";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the /admin/roles listing route. Mirrors the resolved
 * page layout to prevent CLS while the server component fetches the roles list.
 *
 * Reproduces the ListingPageShell geometry manually to avoid wrapping a <Skeleton>
 * block element inside <h1> (which ListingPageShell would do via its title slot).
 */
export function AdminRolesSkeleton() {
  return (
    <ListSkeleton
      className="flex flex-1 flex-col gap-4 sm:gap-6"
      rowCount={3}
      rowClassName="px-4 py-3"
      ariaLabel="Loading roles"
      testId="admin-roles-skeleton"
      header={
        <div className="flex flex-wrap items-end justify-between gap-2">
          <div>
            <Skeleton className="h-8 w-24" />
          </div>
          <div className="flex items-center gap-2">
            <Skeleton className="hidden h-10 w-28 md:inline-flex" />
          </div>
        </div>
      }
      renderRow={() => (
        <div className="flex items-center justify-between">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-8 w-16" />
        </div>
      )}
    />
  );
}
