import { ListSkeleton } from "@/components/layout/list-skeleton";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the cardgroup editor. Mirrors the detail header,
 * card actions, search control, and card rows rendered by the resolved page.
 */
export function CardgroupEditSkeleton() {
  return (
    <ListSkeleton
      ariaLabel="Loading cardgroup editor"
      testId="cardgroup-edit-skeleton"
      rowClassName="overflow-hidden"
      header={
        <>
          <header className="mb-6">
            <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
              <Skeleton className="size-5 justify-self-start" />
              <Skeleton className="h-4 w-20 justify-self-center" />
              <Skeleton className="size-9 justify-self-end" />
            </div>
            <div className="mt-2 flex items-center justify-center">
              <Skeleton className="h-7 w-48 max-w-full" />
            </div>
          </header>

          <div className="mb-3 flex flex-col gap-2 md:flex-row md:items-center md:justify-end">
            <Skeleton className="h-11 w-full md:h-9 md:w-36" />
            <Skeleton className="hidden h-9 w-32 md:block" />
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
