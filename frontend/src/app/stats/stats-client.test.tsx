// @vitest-environment happy-dom
import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { MyLearningStatsQuery } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import jaMessages from "../../../messages/ja.json";
import { StatsClient } from "./stats-client";

// StatsClient is presentational (data via props, no Apollo hooks), so no
// MockedProvider is needed — build a typed fixture and render via renderWithIntl.
type Stats = MyLearningStatsQuery["myLearningStats"];

// A fully-populated stats payload (studied > 0). Numbers are chosen so every
// asserted value is a unique text node (no accidental getByText collisions):
//   acquired = learned + mature = 520 + 300 = 820
//   deck share = (100 + 20) / 500 = 24%  -> aria-valuenow "24"
const populatedStats: Stats = {
  __typename: "LearningStats",
  ownsAnyDeck: true,
  mastery: {
    __typename: "MasteryBreakdown",
    inProgress: 420,
    learned: 520,
    mature: 300,
    totalStudied: 1240,
  },
  decks: [
    {
      __typename: "DeckMastery",
      cardgroup: { __typename: "Cardgroup", id: "cg-1", name: "Spanish Vocab" },
      totalCards: 500,
      learnedCards: 100,
      matureCards: 20,
    },
  ],
  performanceWindows: {
    __typename: "PerformanceWindows",
    days365: {
      __typename: "PerformanceMetrics",
      retentionRate: 0.78,
      successRate: 0.84,
      lapseRate: 0.12,
      studyStreak: 9,
      reviewCount: 1430,
      // Served normalized to 0..1 by the backend; the panel rescales ×10 → "6.2".
      avgDifficulty: 0.62,
    },
    days30: {
      __typename: "PerformanceMetrics",
      retentionRate: 0.75,
      successRate: 0.8,
      lapseRate: 0.15,
      studyStreak: 9,
      reviewCount: 300,
      avgDifficulty: 0.6,
    },
    days7: {
      __typename: "PerformanceMetrics",
      retentionRate: 0.7,
      successRate: 0.76,
      lapseRate: 0.2,
      studyStreak: 9,
      reviewCount: 70,
      avgDifficulty: 0.58,
    },
  },
  strugglingCards: [
    {
      __typename: "StrugglingCard",
      card: {
        __typename: "Card",
        id: "card-1",
        front: "Ephemeral",
        cardgroup: { __typename: "Cardgroup", id: "cg-2", name: "Japanese Kanji" },
      },
      lapses: 7,
    },
  ],
};

const zeroMastery: Stats["mastery"] = {
  __typename: "MasteryBreakdown",
  inProgress: 0,
  learned: 0,
  mature: 0,
  totalStudied: 0,
};

// A deck that exists but has had nothing studied yet (rows render at 0%).
const zeroDeck: Stats["decks"][number] = {
  __typename: "DeckMastery",
  cardgroup: { __typename: "Cardgroup", id: "cg-1", name: "Spanish Vocab" },
  totalCards: 500,
  learnedCards: 0,
  matureCards: 0,
};

function renderStats(stats: Stats, intl?: { locale: "en" | "ja"; messages: typeof jaMessages }) {
  return renderWithIntl(<StatsClient stats={stats} />, intl);
}

describe("<StatsClient>", () => {
  describe("populated (studied > 0)", () => {
    it("renders the acquired headline and the three funnel legend counts", () => {
      renderStats(populatedStats);

      // Big number = acquired = learned + mature = 820.
      expect(screen.getByText("820")).toBeInTheDocument();
      // Funnel legend restates each tier count.
      expect(screen.getByText("420")).toBeInTheDocument(); // in progress
      expect(screen.getByText("520")).toBeInTheDocument(); // learned
      expect(screen.getByText("300")).toBeInTheDocument(); // mature
    });

    it("renders all six diagnostic tiles, each value paired under its own label", () => {
      renderStats(populatedStats);

      // Scope each value to the StatTile that carries its label, so a transposed
      // metric (e.g. retention/success swapped) would fail rather than pass on
      // mere value-presence. The label span's grandparent is the tile card.
      const tile = (label: string) => {
        const card = screen.getByText(label).closest("div")?.parentElement;
        if (!card) throw new Error(`StatTile card not found for "${label}"`);
        return within(card);
      };

      expect(tile("Retention").getByText("78%")).toBeInTheDocument();
      expect(tile("Success").getByText("84%")).toBeInTheDocument();
      expect(tile("Lapse").getByText("12%")).toBeInTheDocument();
      expect(tile("Streak").getByText("9 days")).toBeInTheDocument();
      expect(tile("Reviews").getByText("1,430")).toBeInTheDocument();
      // avgDifficulty is served normalized 0..1 and rescaled ×10 for display.
      expect(tile("Avg difficulty").getByText("6.2")).toBeInTheDocument();
    });

    it("wires a help-hint trigger onto each of the six diagnostic tiles", () => {
      renderStats(populatedStats);
      for (const metric of [
        "Retention",
        "Success",
        "Lapse",
        "Streak",
        "Reviews",
        "Avg difficulty",
      ]) {
        expect(screen.getByRole("button", { name: `About ${metric}` })).toBeInTheDocument();
      }
    });

    it("renders per-deck rows with the acquired/total count and a filled progressbar", () => {
      renderStats(populatedStats);

      expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
      expect(screen.getByText("120/500")).toBeInTheDocument();
      expect(screen.getByText("24%")).toBeInTheDocument();

      const meter = screen.getByRole("progressbar", { name: "Spanish Vocab" });
      expect(meter).toHaveAttribute("aria-valuenow", "24");
    });

    it("renders struggling rows with front, deck name, and lapse count", () => {
      renderStats(populatedStats);

      expect(screen.getByText("Ephemeral")).toBeInTheDocument();
      expect(screen.getByText("Japanese Kanji")).toBeInTheDocument();
      expect(screen.getByText("7 lapses")).toBeInTheDocument();
    });
  });

  describe("ICU plural singular forms", () => {
    it("renders '1 day' for a single-day streak", () => {
      renderStats({
        ...populatedStats,
        performanceWindows: {
          ...populatedStats.performanceWindows,
          days365: { ...populatedStats.performanceWindows.days365, studyStreak: 1 },
        },
      });
      expect(screen.getByText("1 day")).toBeInTheDocument();
    });

    it("renders '1 lapse' for a single-lapse card", () => {
      renderStats({
        ...populatedStats,
        strugglingCards: [
          {
            __typename: "StrugglingCard",
            card: {
              __typename: "Card",
              id: "card-1",
              front: "Ephemeral",
              cardgroup: { __typename: "Cardgroup", id: "cg-2", name: "Japanese Kanji" },
            },
            lapses: 1,
          },
        ],
      });
      expect(screen.getByText("1 lapse")).toBeInTheDocument();
    });
  });

  describe("dormant learner (reviewCount === 0)", () => {
    it("replaces the diagnostic tiles with an empty state (no fabricated rates)", () => {
      renderStats({
        ...populatedStats,
        performanceWindows: {
          ...populatedStats.performanceWindows,
          days365: { ...populatedStats.performanceWindows.days365, reviewCount: 0 },
        },
      });

      expect(screen.getByText("No reviews in the last 365 days.")).toBeInTheDocument();
      // The (placeholder) rate values are not shown as if they were real data.
      expect(screen.queryByText("78%")).not.toBeInTheDocument();
      // The diagnostics heading still renders above the empty state.
      expect(screen.getByText("Diagnostics")).toBeInTheDocument();
    });
  });

  describe("truly-new user (studied === 0 && decks === [] && !ownsAnyDeck)", () => {
    const welcomeStats: Stats = {
      ...populatedStats,
      ownsAnyDeck: false,
      mastery: zeroMastery,
      decks: [],
      strugglingCards: [],
    };

    it("renders only the welcome empty state with both CTAs", () => {
      renderStats(welcomeStats);

      expect(screen.getByText("Your progress starts here")).toBeInTheDocument();
      expect(screen.getByRole("link", { name: "Browse the catalog" })).toHaveAttribute(
        "href",
        "/catalog",
      );
      expect(screen.getByRole("link", { name: "Create a deck" })).toHaveAttribute(
        "href",
        "/cardgroups/new",
      );
      // The empty-deck state is for owners; the truly-new user never sees it.
      expect(screen.queryByText("Your deck has no cards yet")).not.toBeInTheDocument();
    });

    it("collapses every content section (mastery / diagnostics / per-deck / struggling absent)", () => {
      renderStats(welcomeStats);

      expect(screen.queryByText("Words acquired")).not.toBeInTheDocument();
      expect(screen.queryByText("Retention")).not.toBeInTheDocument();
      expect(screen.queryByText("Per-deck acquisition")).not.toBeInTheDocument();
      expect(screen.queryByText("Struggling cards")).not.toBeInTheDocument();
    });
  });

  describe("empty-deck user (studied === 0 && decks === [] && ownsAnyDeck)", () => {
    // Owns at least one cardgroup, but every deck is empty (the backend omits
    // zero-card decks from `decks`), so `decks` is [] yet `ownsAnyDeck` is true.
    const emptyDeckStats: Stats = {
      ...populatedStats,
      ownsAnyDeck: true,
      mastery: zeroMastery,
      decks: [],
      strugglingCards: [],
    };

    it("renders the empty-deck state (add cards CTA), not the welcome state", () => {
      renderStats(emptyDeckStats);

      expect(screen.getByText("Your deck has no cards yet")).toBeInTheDocument();
      expect(screen.getByRole("link", { name: "Add cards" })).toHaveAttribute(
        "href",
        "/cardgroups",
      );
      // Distinguished from the truly-new user: no welcome copy, no catalog CTA.
      expect(screen.queryByText("Your progress starts here")).not.toBeInTheDocument();
      expect(screen.queryByRole("link", { name: "Browse the catalog" })).not.toBeInTheDocument();
    });

    it("collapses every content section (mastery / diagnostics / per-deck / struggling absent)", () => {
      renderStats(emptyDeckStats);

      expect(screen.queryByText("Words acquired")).not.toBeInTheDocument();
      expect(screen.queryByText("Retention")).not.toBeInTheDocument();
      expect(screen.queryByText("Per-deck acquisition")).not.toBeInTheDocument();
      expect(screen.queryByText("Struggling cards")).not.toBeInTheDocument();
    });
  });

  describe("has decks but nothing studied (studied === 0 && decks non-empty)", () => {
    const notStudiedStats: Stats = {
      ...populatedStats,
      mastery: zeroMastery,
      decks: [zeroDeck],
      strugglingCards: [],
    };

    it("renders the not-studied empty state with the start-studying CTA", () => {
      renderStats(notStudiedStats);

      expect(screen.getByText("No progress to show yet")).toBeInTheDocument();
      expect(screen.getByRole("link", { name: "Start studying" })).toHaveAttribute(
        "href",
        "/cardgroups",
      );
    });

    it("still renders per-deck rows at 0% and the struggling empty state", () => {
      renderStats(notStudiedStats);

      expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
      expect(screen.getByText("0/500")).toBeInTheDocument();
      expect(screen.getByRole("progressbar", { name: "Spanish Vocab" })).toHaveAttribute(
        "aria-valuenow",
        "0",
      );
      // No lapses are possible with 0 studied cards.
      expect(screen.getByText("Nothing tricky right now")).toBeInTheDocument();
    });
  });

  describe("populated but no decks (studied > 0 && decks === [])", () => {
    // Reachable transiently: the backend derives the mastery totals and the
    // per-deck totals from separate, non-transactional reads, so a deck deletion
    // landing between them yields studied > 0 alongside an empty deck list.
    const noDecksStats: Stats = { ...populatedStats, decks: [] };

    it("omits the per-deck section instead of rendering it headed-but-empty", () => {
      renderStats(noDecksStats);

      expect(screen.queryByText("Per-deck acquisition")).not.toBeInTheDocument();
      expect(screen.queryByText("Spanish Vocab")).not.toBeInTheDocument();
    });

    it("still renders mastery, diagnostics, and the struggling list", () => {
      renderStats(noDecksStats);

      expect(screen.getByText("820")).toBeInTheDocument();
      expect(screen.getByText("Diagnostics")).toBeInTheDocument();
      expect(screen.getByText("Struggling cards")).toBeInTheDocument();
      expect(screen.getByText("Ephemeral")).toBeInTheDocument();
    });
  });

  describe("populated but no struggling cards (strugglingCards === [])", () => {
    const noStrugglingStats: Stats = { ...populatedStats, strugglingCards: [] };

    it("shows the struggling empty state while the rest of the page renders normally", () => {
      renderStats(noStrugglingStats);

      // Struggling slot collapses to its calm empty state.
      expect(screen.getByText("Nothing tricky right now")).toBeInTheDocument();
      expect(screen.queryByText("Ephemeral")).not.toBeInTheDocument();

      // The rest of the page is unaffected.
      expect(screen.getByText("820")).toBeInTheDocument();
      expect(screen.getByText("Per-deck acquisition")).toBeInTheDocument();
      expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
    });
  });

  describe("localization", () => {
    it("renders the English copy under the default (en) locale", () => {
      renderStats(populatedStats);

      // ListingPageShell title comes from the Stats.title message.
      expect(screen.getByRole("heading", { name: "Progress" })).toBeInTheDocument();
      expect(screen.getByText("Struggling cards")).toBeInTheDocument();
    });

    it("renders the Japanese catalog under the ja locale", () => {
      renderStats(populatedStats, { locale: "ja", messages: jaMessages });

      // The ja catalog replaces the English title + headings (asserted by their
      // absence, without a CJK literal per the language policy). Locale-independent
      // data (the acquired number, the card front) proves the component still
      // rendered under the ja provider.
      expect(screen.queryByText("Progress")).not.toBeInTheDocument();
      expect(screen.queryByText("Struggling cards")).not.toBeInTheDocument();
      expect(screen.getByText("820")).toBeInTheDocument();
      expect(screen.getByText("Ephemeral")).toBeInTheDocument();
    });
  });
});
