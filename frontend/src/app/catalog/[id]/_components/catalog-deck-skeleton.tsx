import { Skeleton } from "@/components/ui/skeleton";

/**
 * Loading placeholder for the `/catalog/[id]` deck-detail route. Mirrors the
 * resolved layout — the DetailPageHeader app bar (back / count / import) with a
 * centered title and badge row, then a single-column read-only card list — to
 * prevent CLS while `CatalogDeckContent` awaits the two parallel GraphQL fetches.
 */
export function CatalogDeckSkeleton() {
  return (
    <main className="p-8">
      <header className="mb-6">
        <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
          <Skeleton className="h-5 w-5 justify-self-start" />
          <Skeleton className="h-5 w-20 justify-self-center" />
          <Skeleton className="h-10 w-28 justify-self-end" />
        </div>
        <div className="mt-1 flex justify-center">
          <Skeleton className="h-8 w-48" />
        </div>
        <div className="mt-2 flex justify-center gap-2">
          <Skeleton className="h-5 w-12" />
          <Skeleton className="h-5 w-16" />
        </div>
      </header>

      <ul
        className="space-y-3"
        aria-busy="true"
        aria-label="Loading deck"
        data-testid="catalog-deck-skeleton"
      >
        {Array.from({ length: 6 }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows have no stable id.
          <li key={i} className="space-y-1 rounded-lg border border-border bg-background px-4 py-3">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
          </li>
        ))}
      </ul>
    </main>
  );
}
