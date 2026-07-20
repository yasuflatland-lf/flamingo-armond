import { getTranslations } from "next-intl/server";
import { CardsNewSkeleton } from "./_components/cards-new-skeleton";

/**
 * Route-segment navigation fallback. Rendered by Next.js while the server
 * component tree for `/cards/new` is being prepared. Wraps the skeleton in
 * the same outer `<main>` / heading markup as `page.tsx` so the page chrome
 * does not shift between the loading state and the resolved page.
 *
 * The heading resolves the same `Cards.newCardTitle` key `page.tsx` renders, so
 * a Japanese visitor does not see an English heading swap to Japanese once the
 * page resolves.
 */
export default async function Loading() {
  const t = await getTranslations("Cards");
  return (
    <main className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">{t("newCardTitle")}</h1>
      <CardsNewSkeleton />
    </main>
  );
}
