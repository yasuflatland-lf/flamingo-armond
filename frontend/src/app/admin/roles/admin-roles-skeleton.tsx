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
    <main className="flex flex-1 flex-col gap-4 p-8 sm:gap-6">
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div>
          <Skeleton className="h-8 w-24" />
        </div>
        <div className="flex items-center gap-2">
          <Skeleton className="hidden h-10 w-28 md:inline-flex" />
        </div>
      </div>

      <ul
        className="space-y-3"
        aria-busy="true"
        aria-label="Loading roles"
        data-testid="admin-roles-skeleton"
      >
        {Array.from({ length: 3 }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows have no stable id.
          <li key={i} className="rounded-md border border-border px-4 py-3">
            <div className="flex items-center justify-between">
              <Skeleton className="h-5 w-32" />
              <Skeleton className="h-8 w-16" />
            </div>
          </li>
        ))}
      </ul>
    </main>
  );
}
