// @vitest-environment happy-dom
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { MyLearningStatsQuery } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { DiagnosticsPanel } from "./diagnostics-panel";

type PerformanceWindows = MyLearningStatsQuery["myLearningStats"]["performanceWindows"];

const performanceWindows: PerformanceWindows = {
  __typename: "PerformanceWindows",
  days365: {
    __typename: "PerformanceMetrics",
    retentionRate: 0.78,
    successRate: 0.84,
    lapseRate: 0.12,
    studyStreak: 9,
    reviewCount: 1430,
    knownReviewCount: 1200,
    avgDifficulty: 0.62,
  },
  days30: {
    __typename: "PerformanceMetrics",
    retentionRate: 0.65,
    successRate: 0.75,
    lapseRate: 0.25,
    studyStreak: 9,
    reviewCount: 300,
    knownReviewCount: 250,
    avgDifficulty: 0.51,
  },
  days7: {
    __typename: "PerformanceMetrics",
    retentionRate: 0.7,
    successRate: 0.8,
    lapseRate: 0.2,
    studyStreak: 9,
    reviewCount: 70,
    knownReviewCount: 60,
    avgDifficulty: 0.48,
  },
};

function renderPanel(windows: PerformanceWindows = performanceWindows) {
  return renderWithIntl(<DiagnosticsPanel performanceWindows={windows} />);
}

function tile(label: string) {
  const card = screen.getByText(label).closest("div")?.parentElement;
  if (!card) throw new Error(`StatTile card not found for "${label}"`);
  return within(card);
}

describe("<DiagnosticsPanel>", () => {
  it("switches snapshots locally and reflects the selected segment", async () => {
    const user = userEvent.setup();
    renderPanel();

    const days365 = screen.getByRole("button", { name: "Last 365 days" });
    const days30 = screen.getByRole("button", { name: "Last 30 days" });
    expect(days365).toHaveAttribute("aria-pressed", "true");
    expect(days30).toHaveAttribute("aria-pressed", "false");
    expect(tile("Retention").getByText("78%")).toBeInTheDocument();

    await user.click(days30);

    expect(days365).toHaveAttribute("aria-pressed", "false");
    expect(days30).toHaveAttribute("aria-pressed", "true");
    expect(tile("Retention").getByText("65%")).toBeInTheDocument();
    expect(tile("Reviews").getByText("300")).toBeInTheDocument();
  });

  it("keeps the switcher available for an empty window and restores tiles", async () => {
    const user = userEvent.setup();
    renderPanel({
      ...performanceWindows,
      // A dormant window carries no gated reviews either — the gated count is a
      // subset of the total, so it can never exceed it.
      days7: { ...performanceWindows.days7, reviewCount: 0, knownReviewCount: 0 },
    });

    await user.click(screen.getByRole("button", { name: "Last 7 days" }));

    expect(screen.getByText("No reviews in the last 7 days.")).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Diagnostics period" })).toBeInTheDocument();
    expect(screen.queryByText("78%")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Last 365 days" }));

    expect(tile("Retention").getByText("78%")).toBeInTheDocument();
    expect(screen.queryByText("No reviews in the last 7 days.")).not.toBeInTheDocument();
  });

  it("replaces the gated rates with an explanatory empty state when no learned-card reviews exist", () => {
    renderPanel({
      ...performanceWindows,
      days365: { ...performanceWindows.days365, knownReviewCount: 0 },
    });

    expect(tile("Retention").getByText("—")).toBeInTheDocument();
    expect(tile("Retention").getByText("No reviews of learned cards yet")).toBeInTheDocument();
    expect(tile("Lapse").getByText("—")).toBeInTheDocument();
    expect(tile("Lapse").getByText("No reviews of learned cards yet")).toBeInTheDocument();

    // Only the two gated tiles change; the whole-window tiles keep their values.
    expect(tile("Success").getByText("84%")).toBeInTheDocument();
    expect(tile("Streak").getByText("9 days")).toBeInTheDocument();
    expect(tile("Reviews").getByText("1,430")).toBeInTheDocument();
  });

  it("falls back to the empty state when knownReviewCount is absent from the result", () => {
    // A query under-selection or an out-of-sync mock leaves this non-null-typed
    // field undefined at runtime; the gate must land on the empty state, never on
    // the misleading "0%" it exists to remove.
    const { knownReviewCount: _absent, ...days365 } = performanceWindows.days365;
    renderPanel({
      ...performanceWindows,
      days365: days365 as PerformanceWindows["days365"],
    });

    expect(tile("Retention").getByText("—")).toBeInTheDocument();
    expect(tile("Retention").getByText("No reviews of learned cards yet")).toBeInTheDocument();
    expect(tile("Lapse").getByText("—")).toBeInTheDocument();
    expect(tile("Lapse").getByText("No reviews of learned cards yet")).toBeInTheDocument();
    expect(screen.queryByText("78%")).not.toBeInTheDocument();
    expect(screen.queryByText("0%")).not.toBeInTheDocument();
  });

  it("renders the gated rates as percentages when learned-card reviews exist", () => {
    renderPanel();

    expect(tile("Retention").getByText("78%")).toBeInTheDocument();
    expect(tile("Retention").getByText("recall success")).toBeInTheDocument();
    expect(tile("Lapse").getByText("12%")).toBeInTheDocument();
    expect(tile("Lapse").getByText("forgotten again")).toBeInTheDocument();
    expect(screen.queryByText("No reviews of learned cards yet")).not.toBeInTheDocument();
  });

  it("shows the same full-history streak in every window", async () => {
    const user = userEvent.setup();
    renderPanel();

    expect(tile("Streak").getByText("9 days")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Last 30 days" }));
    expect(tile("Streak").getByText("9 days")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Last 7 days" }));
    expect(tile("Streak").getByText("9 days")).toBeInTheDocument();
  });
});
