"use client";

import { useFormatter, useTranslations } from "next-intl";
import type { MyLearningStatsQuery } from "@/generated/graphql";
import { cn } from "@/lib/utils";

type Mastery = MyLearningStatsQuery["myLearningStats"]["mastery"];

/**
 * Hero card for `/stats`: the acquired-words headline plus a three-tier mastery
 * funnel (in-progress → learned → mature). The funnel segments are weighted by
 * `flexGrow` so their widths encode the tier counts; a zero-count tier collapses
 * to nothing. A legend row under the bar restates each tier's colour, label, and
 * count. Only rendered when the learner has studied at least one card.
 */
export function MasterySummary({ mastery }: { mastery: Mastery }) {
  const t = useTranslations("Stats");
  const format = useFormatter();
  const acquired = mastery.learned + mastery.mature;

  const tiers = [
    {
      key: "inProgress",
      label: t("masteryInProgress"),
      count: mastery.inProgress,
      color: "bg-muted-foreground/30",
    },
    {
      key: "learned",
      label: t("masteryLearned"),
      count: mastery.learned,
      color: "bg-brand-primary/50",
    },
    { key: "mature", label: t("masteryMature"), count: mastery.mature, color: "bg-brand-primary" },
  ];

  return (
    <section className="rounded-xl border bg-card p-4 text-card-foreground shadow-sm">
      <div className="flex flex-col gap-6 md:flex-row md:items-center">
        <div className="md:shrink-0">
          <p className="text-xs font-medium text-muted-foreground">{t("masteryLabel")}</p>
          <p className="text-4xl font-bold tabular-nums">{format.number(acquired)}</p>
          <p className="text-sm text-muted-foreground tabular-nums">
            {t("masteryAcquiredOf", { total: mastery.totalStudied })}
          </p>
        </div>
        <div className="flex-1">
          <div className="flex h-3 w-full overflow-hidden rounded-full bg-muted">
            {tiers.map((tier) => (
              <div key={tier.key} className={tier.color} style={{ flexGrow: tier.count }} />
            ))}
          </div>
          <ul className="mt-3 flex flex-wrap gap-x-4 gap-y-1">
            {tiers.map((tier) => (
              <li key={tier.key} className="flex items-center gap-1.5 text-xs">
                <span aria-hidden="true" className={cn("h-2 w-2 rounded-full", tier.color)} />
                <span className="text-muted-foreground">{tier.label}</span>
                <span className="font-medium tabular-nums">{format.number(tier.count)}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </section>
  );
}
