"use client";

import { useFormatter, useTranslations } from "next-intl";
import { ProgressMeter } from "@/components/ui/progress-meter";
import type { MyLearningStatsQuery } from "@/generated/graphql";

type Decks = MyLearningStatsQuery["myLearningStats"]["decks"];

/**
 * Per-deck acquisition list for `/stats`. One row per owned deck (with at least
 * one card): the deck name, a `ProgressMeter` filled to `acquired / totalCards`,
 * the raw `acquired/total` count, and the whole-percent share. `acquired` counts
 * Learned + Mature cards (the two are disjoint on the backend). `totalCards === 0`
 * guards to 0% (the meter itself also clamps a zero denominator).
 */
export function PerDeckList({ decks }: { decks: Decks }) {
  const t = useTranslations("Stats");
  const format = useFormatter();

  return (
    <section className="rounded-xl border bg-card p-4 text-card-foreground shadow-sm">
      <h2 className="text-sm font-semibold">{t("perDeckHeading")}</h2>
      <ul className="mt-4 flex flex-col gap-4">
        {decks.map((deck) => {
          const acquired = deck.learnedCards + deck.matureCards;
          const share = deck.totalCards > 0 ? acquired / deck.totalCards : 0;
          return (
            <li key={deck.cardgroup.id} className="flex flex-col gap-1.5">
              <div className="flex items-baseline justify-between gap-2">
                <span className="truncate text-sm font-medium">{deck.cardgroup.name}</span>
                <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                  {format.number(share, { style: "percent", maximumFractionDigits: 0 })}
                </span>
              </div>
              <ProgressMeter value={acquired} max={deck.totalCards} label={deck.cardgroup.name} />
              <span className="text-xs text-muted-foreground tabular-nums">
                {t("perDeckAcquiredOfTotal", { acquired, total: deck.totalCards })}
              </span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
