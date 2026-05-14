import { LearnSkeleton } from "./_components/learn-skeleton";

/**
 * Route-segment navigation fallback. Rendered by Next.js while the server
 * component tree for `/learn/[cardgroupId]` is being prepared. Wraps the
 * skeleton in the same outer `<main>` / container markup as `page.tsx` so the
 * page layout does not shift between the loading state and the resolved page.
 */
export default function Loading() {
  return (
    <main className="flex min-h-0 flex-1 flex-col bg-background">
      <div className="mx-auto flex min-h-0 w-full max-w-5xl flex-1 flex-col p-4">
        <LearnSkeleton />
      </div>
    </main>
  );
}
