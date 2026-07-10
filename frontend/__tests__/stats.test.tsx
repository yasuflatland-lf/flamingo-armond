// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// ---------------------------------------------------------------------------
// Module mocks — hoisted by Vitest before imports
// ---------------------------------------------------------------------------

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`REDIRECT:${path}`);
  }),
}));

// ---------------------------------------------------------------------------
// Imports — after vi.mock declarations
// ---------------------------------------------------------------------------

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import StatsPage from "@/app/stats/page";
import type { MyLearningStatsQuery } from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { renderWithIntl } from "@/test/render-with-intl";

// ---------------------------------------------------------------------------
// Fixtures / helpers
// ---------------------------------------------------------------------------

// A populated payload (studied > 0). The broad test only asserts page
// composition (auth gate + fetch wiring + the rendered "Progress" title);
// flow-detail assertions live in the co-located stats-client.test.tsx.
const populatedStats: MyLearningStatsQuery["myLearningStats"] = {
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
  performance: {
    __typename: "PerformanceMetrics",
    retentionRate: 0.78,
    successRate: 0.84,
    lapseRate: 0.12,
    studyStreak: 9,
    reviewCount: 1430,
    // Served normalized to 0..1 by the backend (rescaled ×10 for display).
    avgDifficulty: 0.62,
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

function mockGqlFetch(data: unknown): void {
  vi.mocked(gqlFetch).mockResolvedValue(data as never);
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("StatsPage", () => {
  it("renders the Progress title when authenticated with a populated result", async () => {
    mockGqlFetch({ myLearningStats: populatedStats });

    const tree = await StatsPage();
    renderWithIntl(tree as React.ReactElement);

    expect(screen.getByRole("heading", { name: "Progress" })).toBeInTheDocument();
    expect(redirect).not.toHaveBeenCalled();
  });

  it("redirects to /login when the caller is unauthenticated", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "anonymous" }));

    await expect(StatsPage()).rejects.toThrow("REDIRECT:/login");

    expect(redirect).toHaveBeenCalledWith("/login");
    // The auth gate short-circuits before any data fetch.
    expect(gqlFetch).not.toHaveBeenCalled();
  });
});
