import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for `/profile/change-email`. Mirrors the resolved
 * heading and email form so the streamed controls keep their final positions.
 */
export function ChangeEmailSkeleton() {
  return (
    <main
      className="p-8"
      aria-busy="true"
      aria-label="Loading change email form"
      data-testid="change-email-skeleton"
    >
      <Skeleton className="mb-6 h-8 w-48 max-w-full" />

      <div className="space-y-4">
        <div className="space-y-2">
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-6 w-48 max-w-full" />
        </div>

        <div className="space-y-2">
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-10 w-full" />
        </div>

        <div className="flex gap-2">
          <Skeleton className="h-10 w-44" />
          <Skeleton className="h-10 w-20" />
        </div>
      </div>
    </main>
  );
}
