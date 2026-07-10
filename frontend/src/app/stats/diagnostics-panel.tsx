"use client";

import { CircleCheck, Flame, Gauge, Repeat, Target, TriangleAlert } from "lucide-react";
import { useFormatter, useTranslations } from "next-intl";
import { StatTile } from "@/components/ui/stat-tile";
import type { MyLearningStatsQuery } from "@/generated/graphql";

// Aliased to the GraphQL type name (not the bare word `Performance`, which would
// shadow the ambient DOM `Performance` interface in this DOM-lib file).
type PerformanceMetrics = MyLearningStatsQuery["myLearningStats"]["performance"];

// Shared by the three rate tiles (retention / success / lapse) below — all three
// format a 0..1 fraction as a whole-percent string.
const PERCENT_FORMAT = { style: "percent", maximumFractionDigits: 0 } as const;

/**
 * Diagnostics KPI row for `/stats`: six `StatTile`s over the trailing-365-day
 * window. Rate metrics (`retentionRate` / `successRate` / `lapseRate`) are
 * fractions 0..1 rendered as whole-percent via `useFormatter`. `avgDifficulty`
 * is normalized by the backend to 0..1 (see `service.normalizedDifficulty`), so
 * it is rescaled ×10 for display on the familiar 0–10 FSRS difficulty scale,
 * shown to one decimal. `studyStreak` uses the pluralized `diagnosticsStreakValue`
 * message; `reviewCount` is a grouped integer. When `reviewCount === 0` (a dormant
 * learner with no reviews in the window) the backend returns placeholder rate
 * values, so the tiles are replaced by an empty state instead of showing fabricated
 * data as real.
 */
export function DiagnosticsPanel({ performance }: { performance: PerformanceMetrics }) {
  const t = useTranslations("Stats");
  const format = useFormatter();

  const tiles = [
    {
      key: "retention",
      label: t("diagnosticsRetention"),
      value: format.number(performance.retentionRate, PERCENT_FORMAT),
      caption: t("diagnosticsRetentionCaption"),
      icon: Target,
    },
    {
      key: "success",
      label: t("diagnosticsSuccess"),
      value: format.number(performance.successRate, PERCENT_FORMAT),
      caption: t("diagnosticsSuccessCaption"),
      icon: CircleCheck,
    },
    {
      key: "lapse",
      label: t("diagnosticsLapse"),
      value: format.number(performance.lapseRate, PERCENT_FORMAT),
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
      // Backend serves this normalized to 0..1; rescale ×10 to the 0–10 FSRS scale.
      value: format.number(performance.avgDifficulty * 10, { maximumFractionDigits: 1 }),
      caption: t("diagnosticsAvgDifficultyCaption"),
      icon: Gauge,
    },
  ];

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between gap-2">
        <h2 className="text-sm font-semibold">{t("diagnosticsHeading")}</h2>
        <span className="shrink-0 text-xs text-muted-foreground">{t("diagnosticsWindow")}</span>
      </div>
      {performance.reviewCount === 0 ? (
        // Dormant learner: no reviews in the window. The backend returns
        // placeholder rate values for an empty window, so show an empty state
        // rather than render fabricated data as if it were real.
        <p className="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
          {t("diagnosticsNoActivity")}
        </p>
      ) : (
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
      )}
    </div>
  );
}
