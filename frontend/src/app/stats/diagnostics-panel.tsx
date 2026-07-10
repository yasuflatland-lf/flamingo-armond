"use client";

import { CircleCheck, Flame, Gauge, Repeat, Target, TriangleAlert } from "lucide-react";
import { useFormatter, useTranslations } from "next-intl";
import { StatTile } from "@/components/ui/stat-tile";
import type { MyLearningStatsQuery } from "@/generated/graphql";

type Performance = MyLearningStatsQuery["myLearningStats"]["performance"];

/**
 * Diagnostics KPI row for `/stats`: six `StatTile`s over the trailing-365-day
 * window. Rate metrics (`retentionRate` / `successRate` / `lapseRate`) are
 * fractions 0..1 rendered as whole-percent via `useFormatter`; `avgDifficulty`
 * is a raw FSRS float shown to one decimal; `studyStreak` uses the pluralized
 * `streakValue` message; `reviewCount` is a grouped integer.
 */
export function DiagnosticsPanel({ performance }: { performance: Performance }) {
  const t = useTranslations("Stats");
  const format = useFormatter();

  const tiles = [
    {
      key: "retention",
      label: t("diagnosticsRetention"),
      value: format.number(performance.retentionRate, {
        style: "percent",
        maximumFractionDigits: 0,
      }),
      caption: t("diagnosticsRetentionCaption"),
      icon: Target,
    },
    {
      key: "success",
      label: t("diagnosticsSuccess"),
      value: format.number(performance.successRate, {
        style: "percent",
        maximumFractionDigits: 0,
      }),
      caption: t("diagnosticsSuccessCaption"),
      icon: CircleCheck,
    },
    {
      key: "lapse",
      label: t("diagnosticsLapse"),
      value: format.number(performance.lapseRate, {
        style: "percent",
        maximumFractionDigits: 0,
      }),
      caption: t("diagnosticsLapseCaption"),
      icon: TriangleAlert,
    },
    {
      key: "streak",
      label: t("diagnosticsStreak"),
      value: t("diagnosticsStreakValue", { days: performance.studyStreak }),
      caption: t("diagnosticsStreakCaption"),
      icon: Flame,
    },
    {
      key: "reviews",
      label: t("diagnosticsReviews"),
      value: format.number(performance.reviewCount),
      caption: t("diagnosticsReviewsCaption"),
      icon: Repeat,
    },
    {
      key: "avgDifficulty",
      label: t("diagnosticsAvgDifficulty"),
      value: format.number(performance.avgDifficulty, { maximumFractionDigits: 1 }),
      caption: t("diagnosticsAvgDifficultyCaption"),
      icon: Gauge,
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
      {tiles.map((tile) => (
        <StatTile
          key={tile.key}
          label={tile.label}
          value={tile.value}
          caption={tile.caption}
          icon={tile.icon}
        />
      ))}
    </div>
  );
}
