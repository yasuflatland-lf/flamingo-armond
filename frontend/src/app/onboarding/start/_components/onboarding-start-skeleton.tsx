import { Skeleton } from "@/components/ui/skeleton";

function DeckTileSkeleton() {
  return (
    <li className="flex h-40 w-[17rem] flex-col gap-3 rounded-xl border border-border bg-card p-5 shadow-sm">
      <Skeleton className="h-5 w-3/4" />
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-16" />
      <Skeleton className="mt-auto h-9 w-full" />
    </li>
  );
}

/**
 * Loading placeholder for `/onboarding/start`. Mirrors the preset chooser hero
 * while retaining the bare-shell route's main landmark.
 */
export function OnboardingStartSkeleton() {
  return (
    <main
      aria-busy="true"
      aria-label="Loading onboarding choices"
      data-testid="onboarding-start-skeleton"
    >
      <div className="mx-auto max-w-2xl px-6 py-12 text-center sm:py-16">
        <Skeleton className="mx-auto size-12 rounded-full" />
        <Skeleton className="mx-auto mt-5 h-8 w-72 max-w-full" />
        <Skeleton className="mx-auto mt-2 h-5 w-96 max-w-full" />

        <section className="mt-11">
          <div className="mx-auto w-fit max-w-full rounded-2xl border border-brand-tint-border bg-brand-tint p-6 text-left">
            <Skeleton className="h-4 w-28" />
            <ul className="mt-4 flex flex-wrap justify-center gap-4 sm:gap-5">
              <DeckTileSkeleton />
              <DeckTileSkeleton />
            </ul>
          </div>
        </section>

        <Skeleton className="mx-auto mt-8 h-5 w-52 max-w-full" />
      </div>
    </main>
  );
}
