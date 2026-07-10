"use client";

import { useTranslations } from "next-intl";
import type { ReactNode } from "react";
import { ListingPageShell } from "@/components/layout/listing-page-shell";
import type { MyLearningStatsQuery } from "@/generated/graphql";
import { DiagnosticsPanel } from "./diagnostics-panel";
import { MasterySummary } from "./mastery-summary";
import { PerDeckList } from "./per-deck-list";
import { NotStudiedEmpty, StrugglingEmpty, WelcomeEmpty } from "./stats-empty-states";
import { StrugglingList } from "./struggling-list";

/**
 * Presentational shell for `/stats`. Data arrives fully-fetched from the RSC
 * (`page.tsx`) as a prop — no Apollo hooks here — and this component only routes
 * the three data-absence cases and lays out the four content sections. All copy
 * flows through `useTranslations("Stats")`; number/percent formatting lives in
 * the section components via `useFormatter()`.
 */
export function StatsClient({ stats }: { stats: MyLearningStatsQuery["myLearningStats"] }) {
  const t = useTranslations("Stats");
  const studiedCount = stats.mastery.totalStudied;
  const hasDecks = stats.decks.length > 0;

  let content: ReactNode;
  if (studiedCount === 0 && !hasDecks) {
    // Brand-new user: owns no deck that has any cards (the backend omits
    // zero-card decks from `decks`), and nothing studied. All sections collapse.
    content = <WelcomeEmpty />;
  } else if (studiedCount === 0) {
    // Has decks but no reviews yet: per-deck rows render at 0%, and the
    // struggling slot is necessarily empty (no lapses without studied cards) —
    // rendered directly here, mirroring StrugglingList's own empty-cards guard.
    content = (
      <div className="flex flex-col gap-4 sm:gap-6">
        <NotStudiedEmpty />
        <div className="grid gap-4 lg:grid-cols-[4fr_3fr]">
          <PerDeckList decks={stats.decks} />
          <StrugglingEmpty />
        </div>
      </div>
    );
  } else {
    content = (
      <div className="flex flex-col gap-4 sm:gap-6">
        <MasterySummary mastery={stats.mastery} />
        <DiagnosticsPanel performance={stats.performance} />
        <div className="grid gap-4 lg:grid-cols-[4fr_3fr]">
          <PerDeckList decks={stats.decks} />
          <StrugglingList cards={stats.strugglingCards} />
        </div>
      </div>
    );
  }

  return (
    <ListingPageShell title={t("title")} description={t("subtitle")}>
      {content}
    </ListingPageShell>
  );
}
