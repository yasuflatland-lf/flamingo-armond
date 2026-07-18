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
    avgDifficulty: 0.62,
  },
  days30: {
    __typename: "PerformanceMetrics",
    retentionRate: 0.65,
    successRate: 0.75,
    lapseRate: 0.25,
    studyStreak: 9,
    reviewCount: 300,
    avgDifficulty: 0.51,
  },
  days7: {
    __typename: "PerformanceMetrics",
    retentionRate: 0.7,
    successRate: 0.8,
    lapseRate: 0.2,
    studyStreak: 9,
    reviewCount: 70,
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
      days7: { ...performanceWindows.days7, reviewCount: 0 },
    });

    await user.click(screen.getByRole("button", { name: "Last 7 days" }));

    expect(screen.getByText("No reviews in the last 7 days.")).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Diagnostics period" })).toBeInTheDocument();
    expect(screen.queryByText("78%")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Last 365 days" }));

    expect(tile("Retention").getByText("78%")).toBeInTheDocument();
    expect(screen.queryByText("No reviews in the last 7 days.")).not.toBeInTheDocument();
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

  it("hides unavailable short-window controls during a legacy-backend rollout", () => {
    renderWithIntl(
      <DiagnosticsPanel performanceWindows={performanceWindows} showWindowSelector={false} />,
    );

    expect(screen.queryByRole("group", { name: "Diagnostics period" })).not.toBeInTheDocument();
    expect(tile("Reviews").getByText("1,430")).toBeInTheDocument();
  });
});
