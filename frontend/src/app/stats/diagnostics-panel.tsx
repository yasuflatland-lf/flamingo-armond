"use client";

import { useFormatter, useTranslations } from "next-intl";
import { useState } from "react";
import { EmptyState } from "@/components/ui/empty-state";
import { StatTile } from "@/components/ui/stat-tile";
import type { MyLearningStatsQuery } from "@/generated/graphql";
import { cn } from "@/lib/utils";
import { StatHint } from "./stat-hint";

type PerformanceWindows = MyLearningStatsQuery["myLearningStats"]["performanceWindows"];
type WindowKey = "days365" | "days30" | "days7";

const WINDOW_OPTIONS: ReadonlyArray<{ key: WindowKey; days: number }> = [
  { key: "days365", days: 365 },
  { key: "days30", days: 30 },
  { key: "days7", days: 7 },
];

// Shared by the three rate tiles (retention / success / lapse) below — all three
// format a 0..1 fraction as a whole-percent string.
const PERCENT_FORMAT = { style: "percent", maximumFractionDigits: 0 } as const;

/**
 * Diagnostics KPI row for `/stats`: six `StatTile`s over a locally selected
 * trailing window, each carrying a `StatHint` help popover. Rate
 * metrics (`retentionRate` / `successRate` / `lapseRate`) are fractions 0..1
 * rendered as whole-percent via `useFormatter`. `avgDifficulty` is normalized by
 * the backend to 0..1 (see `service.normalizedDifficulty`), so it is rescaled ×10
 * for display on the familiar 0–10 FSRS difficulty scale, shown to one decimal.
 * `studyStreak` uses the pluralized `diagnosticsStreakValue` message; `reviewCount`
 * is a grouped integer. Switching windows is synchronous because all snapshots
 * arrive in one query. When the selected window has no reviews, only the tile
 * area becomes an empty state so the window control remains available.
 */
export function DiagnosticsPanel({
  performanceWindows,
  showWindowSelector = true,
}: {
  performanceWindows: PerformanceWindows;
  showWindowSelector?: boolean;
}) {
  const t = useTranslations("Stats");
  const format = useFormatter();
  const [selectedWindow, setSelectedWindow] = useState<WindowKey>("days365");
  const performance = performanceWindows[selectedWindow];
  const selectedDays = WINDOW_OPTIONS.find(({ key }) => key === selectedWindow)?.days ?? 365;

  const tiles = [
    {
      key: "retention",
      label: t("diagnosticsRetention"),
      value: format.number(performance.retentionRate, PERCENT_FORMAT),
      caption: t("diagnosticsRetentionCaption"),
      hint: t("diagnosticsRetentionHint"),
    },
    {
      key: "success",
      label: t("diagnosticsSuccess"),
      value: format.number(performance.successRate, PERCENT_FORMAT),
      caption: t("diagnosticsSuccessCaption"),
      hint: t("diagnosticsSuccessHint"),
    },
    {
      key: "lapse",
      label: t("diagnosticsLapse"),
      value: format.number(performance.lapseRate, PERCENT_FORMAT),
      caption: t("diagnosticsLapseCaption"),
      hint: t("diagnosticsLapseHint"),
    },
    {
      key: "streak",
      label: t("diagnosticsStreak"),
      value: t("diagnosticsStreakValue", { days: performance.studyStreak }),
      caption: t("diagnosticsStreakCaption"),
      hint: t("diagnosticsStreakHint"),
    },
    {
      key: "reviews",
      label: t("diagnosticsReviews"),
      value: format.number(performance.reviewCount),
      caption: t("diagnosticsReviewsCaption"),
      hint: t("diagnosticsReviewsHint", { days: selectedDays }),
    },
    {
      key: "avgDifficulty",
      label: t("diagnosticsAvgDifficulty"),
      // Backend serves this normalized to 0..1; rescale ×10 to the 0–10 FSRS scale.
      value: format.number(performance.avgDifficulty * 10, { maximumFractionDigits: 1 }),
      caption: t("diagnosticsAvgDifficultyCaption"),
      hint: t("diagnosticsAvgDifficultyHint"),
    },
  ];

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold">{t("diagnosticsHeading")}</h2>
        {showWindowSelector ? (
          <fieldset
            aria-label={t("diagnosticsWindowGroupAria")}
            className="inline-flex min-w-0 shrink-0 rounded-lg border-0 bg-muted p-0.5"
          >
            {WINDOW_OPTIONS.map(({ key, days }) => {
              const selected = key === selectedWindow;
              return (
                <button
                  key={key}
                  type="button"
                  aria-label={t("diagnosticsWindowOptionAria", { days })}
                  aria-pressed={selected}
                  onClick={() => setSelectedWindow(key)}
                  className={cn(
                    "min-h-11 min-w-11 rounded-md px-3 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
                    selected
                      ? "bg-brand-primary text-brand-primary-foreground"
                      : "text-muted-foreground hover:text-foreground",
                  )}
                >
                  {t("diagnosticsWindowOption", { days })}
                </button>
              );
            })}
          </fieldset>
        ) : null}
      </div>
      {performance.reviewCount === 0 ? (
        // Dormant learner: no reviews in the window. The backend returns
        // placeholder rate values for an empty window, so show an empty state
        // rather than render fabricated data as if it were real.
        <EmptyState
          className="p-6"
          body={t("diagnosticsNoActivityInWindow", { days: selectedDays })}
        />
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          {tiles.map((tile) => (
            <StatTile
              key={tile.key}
              label={tile.label}
              value={tile.value}
              caption={tile.caption}
              action={
                <StatHint
                  title={tile.label}
                  body={tile.hint}
                  ariaLabel={t("diagnosticsHintAriaLabel", { metric: tile.label })}
                />
              }
            />
          ))}
        </div>
      )}
    </div>
  );
}
