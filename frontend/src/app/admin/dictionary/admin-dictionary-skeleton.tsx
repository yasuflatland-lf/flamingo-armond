import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the /admin/dictionary route. Mirrors the resolved
 * page layout to prevent CLS while the server component runs auth checks.
 */
export function AdminDictionarySkeleton() {
  return (
    <main className="p-8">
      <Skeleton className="mb-6 h-8 w-56" />

      <div className="space-y-6">
        {/* Cardgroup selector */}
        <div className="space-y-2">
          <Skeleton className="h-4 w-36" />
          <Skeleton className="h-10 w-full max-w-sm" />
        </div>

        {/* Payload textarea */}
        <div className="space-y-2">
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-3 w-72" />
          <Skeleton className="h-40 w-full" />
        </div>

        {/* Action buttons */}
        <div className="flex gap-3">
          <Skeleton className="h-10 w-24" />
          <Skeleton className="h-10 w-20" />
        </div>
      </div>
    </main>
  );
}
