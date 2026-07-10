"use client";

import { useTranslations } from "next-intl";
import type { MyLearningStatsQuery } from "@/generated/graphql";
import { StrugglingEmpty } from "./stats-empty-states";

type StrugglingCards = MyLearningStatsQuery["myLearningStats"]["strugglingCards"];

/**
 * Struggling-card list for `/stats`: the cards the learner lapses on most,
 * pre-ranked and capped at 10 by the backend. Each row shows the card front, its
 * owning deck name, and the lapse count — no `back`, no `stability` (per the
 * approved design). Renders `StrugglingEmpty` when there are no struggling cards.
 */
export function StrugglingList({ cards }: { cards: StrugglingCards }) {
  const t = useTranslations("Stats");

  if (cards.length === 0) {
    return <StrugglingEmpty />;
  }

  return (
    <section className="rounded-xl border bg-card p-4 text-card-foreground shadow-sm">
      <h2 className="text-sm font-semibold">{t("strugglingHeading")}</h2>
      <ul className="mt-4 flex flex-col gap-3">
        {cards.map((sc) => (
          <li
            key={sc.card.id}
            className="flex items-center justify-between gap-3 rounded-lg border border-border bg-background px-4 py-3"
          >
            <div className="min-w-0">
              <p className="truncate text-sm font-medium">{sc.card.front}</p>
              <p className="truncate text-xs text-muted-foreground">{sc.card.cardgroup.name}</p>
            </div>
            <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
              {t("strugglingLapses", { count: sc.lapses })}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
