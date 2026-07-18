// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { MyLearningStatsQuery } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { PerDeckList } from "./per-deck-list";

type Decks = MyLearningStatsQuery["myLearningStats"]["decks"];

function deck(overrides: Partial<Decks[number]> & { id: string; name: string }): Decks[number] {
  const { id, name, ...rest } = overrides;
  return {
    __typename: "DeckMastery",
    cardgroup: { __typename: "Cardgroup", id, name },
    totalCards: 0,
    learnedCards: 0,
    matureCards: 0,
    ...rest,
  };
}

describe("<PerDeckList>", () => {
  it("renders the acquired/total count, the whole-percent share, and a filled meter", () => {
    renderWithIntl(
      <PerDeckList
        decks={[
          deck({
            id: "cg-1",
            name: "Spanish Vocab",
            totalCards: 500,
            learnedCards: 100,
            matureCards: 20,
          }),
        ]}
      />,
    );

    expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
    // acquired = learned + mature = 120, total = 500.
    expect(screen.getByText("120/500")).toBeInTheDocument();
    // share = 120 / 500 = 24%.
    expect(screen.getByText("24%")).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "Spanish Vocab" })).toHaveAttribute(
      "aria-valuenow",
      "24",
    );
  });

  it("clamps the count and meter to totalCards when a non-transactional snapshot overshoots", () => {
    // The backend reads FSRS states and deck totals in two separate queries, so a
    // concurrent card deletion can leave acquired (5) above totalCards (2). The row
    // must render "2/2" at 100%, never the raw "5/2".
    renderWithIntl(
      <PerDeckList
        decks={[
          deck({
            id: "cg-1",
            name: "Spanish Vocab",
            totalCards: 2,
            learnedCards: 3,
            matureCards: 2,
          }),
        ]}
      />,
    );

    expect(screen.getByText("2/2")).toBeInTheDocument();
    expect(screen.queryByText("5/2")).not.toBeInTheDocument();
    expect(screen.getByText("100%")).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "Spanish Vocab" })).toHaveAttribute(
      "aria-valuenow",
      "100",
    );
  });
});
