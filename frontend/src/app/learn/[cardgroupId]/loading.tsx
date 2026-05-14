import { LearnSkeleton } from "./_components/learn-skeleton";

// Mirrors the outer <main> / container from page.tsx so layout does not shift
// between the navigation loading state and the resolved page.
export default function Loading() {
  return (
    <main className="flex min-h-0 flex-1 flex-col bg-background">
      <div className="mx-auto flex min-h-0 w-full max-w-5xl flex-1 flex-col p-4">
        <LearnSkeleton />
      </div>
    </main>
  );
}
