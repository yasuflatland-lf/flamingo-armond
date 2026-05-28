// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CardContent, type SwipeCardData } from "./swipe-card";

const CARD: SwipeCardData = {
  id: "card-1",
  front: "Hello",
  back: "Hola",
  userCardState: {
    due: "2026-04-30T00:00:00Z",
    state: 0,
  },
  cardgroupId: "cg-1",
};

describe("<CardContent>", () => {
  it("renders the card front and back", () => {
    render(<CardContent card={CARD} />);

    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("Hola")).toBeInTheDocument();
  });

  it("does not render rating buttons inside the card", () => {
    render(<CardContent card={CARD} />);

    expect(screen.queryByRole("button", { name: "Again" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Hard" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Easy" })).not.toBeInTheDocument();
  });

  it("renders a mastery badge for state 2 (Learned)", () => {
    const learnedCard: SwipeCardData = {
      ...CARD,
      userCardState: { ...CARD.userCardState, state: 2 },
    };
    render(<CardContent card={learnedCard} />);

    expect(screen.getByLabelText("Mastery stage: Learned")).toBeInTheDocument();
  });
});
