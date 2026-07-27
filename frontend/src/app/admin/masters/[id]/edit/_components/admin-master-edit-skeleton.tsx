import { ListSkeleton } from "@/components/layout/list-skeleton";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the master editor. Mirrors the detail header,
 * desktop card actions, search control, and card rows of the resolved page.
 */
export function AdminMasterEditSkeleton() {
  return (
    <ListSkeleton
      ariaLabel="Loading master editor"
      testId="admin-master-edit-skeleton"
      rowClassName="overflow-hidden"
      header={
        <>
          <header className="mb-6">
            <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
              <Skeleton className="size-5 justify-self-start" />
              <Skeleton className="h-4 w-20 justify-self-center" />
              <Skeleton className="size-9 justify-self-end md:hidden" />
              <Skeleton className="hidden h-10 w-36 justify-self-end md:block" />
            </div>
            <div className="mt-2 flex items-center justify-center">
              <Skeleton className="h-7 w-48 max-w-full" />
            </div>
            <div className="mt-1 flex items-center justify-center">
              <Skeleton className="h-6 w-20 rounded-full" />
            </div>
          </header>

          <div className="mb-3 hidden justify-end md:flex">
            <Skeleton className="h-9 w-32" />
          </div>

          <Skeleton className="mb-3 hidden h-10 w-full md:block" />
        </>
      }
      renderRow={() => (
        <div className="flex items-center justify-between gap-4 px-4 py-3">
          <Skeleton className="size-4 shrink-0" />
          <div className="min-w-0 flex-1 space-y-1">
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-4 w-1/2" />
          </div>
          <Skeleton className="hidden size-8 shrink-0 sm:block" />
        </div>
      )}
    />
  );
}
