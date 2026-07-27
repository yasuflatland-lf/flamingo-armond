import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for `/onboarding`. Mirrors the centered hero and profile
 * form while retaining the bare-shell route's main landmark.
 */
export function OnboardingSkeleton() {
  return (
    <main aria-busy="true" aria-label="Loading onboarding" data-testid="onboarding-skeleton">
      <div className="mx-auto max-w-2xl px-6 py-12 text-center sm:py-16">
        <Skeleton className="mx-auto size-12 rounded-full" />
        <Skeleton className="mx-auto mt-5 h-8 w-64 max-w-full" />
        <Skeleton className="mx-auto mt-2 h-5 w-80 max-w-full" />

        <div className="mx-auto mt-8 w-full max-w-[420px] space-y-4 rounded-2xl border border-border/70 bg-card p-6 text-left shadow-[0_1px_2px_rgba(0,0,0,0.04),0_12px_32px_-12px_rgba(0,0,0,0.12)] sm:p-8">
          <div className="space-y-2">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-4 w-3/4" />
          </div>
          <Skeleton className="h-10 w-full" />
        </div>
      </div>
    </main>
  );
}
