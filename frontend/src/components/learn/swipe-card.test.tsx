// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CardContent, type SwipeCardData } from "./swipe-card";

const CARD: SwipeCardData = {
  id: "card-1",
  front: "Hello",
  back: "Hola",
  cefrLevel: null,
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

  it("renders the CEFR badge alongside the front/back when the card has a level", () => {
    render(<CardContent card={{ ...CARD, cefrLevel: "B1" }} />);

    expect(screen.getByLabelText("CEFR level B1")).toBeInTheDocument();
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("Hola")).toBeInTheDocument();
  });

  it("renders no CEFR badge when the level is null, but still shows the front/back", () => {
    render(<CardContent card={{ ...CARD, cefrLevel: null }} />);

    expect(screen.queryByLabelText(/CEFR level/)).toBeNull();
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("Hola")).toBeInTheDocument();
  });

  it("reserves horizontal padding so a long front term cannot slide under the badge", () => {
    // jsdom has no real layout engine, so the overlap is guarded structurally:
    // the centered content block must carry the horizontal-padding utility that
    // keeps a pathological single-token `front` clear of the right-pinned badge.
    const longFront = "Pneumonoultramicroscopicsilicovolcanoconiosis";
    render(<CardContent card={{ ...CARD, front: longFront, cefrLevel: "C1" }} />);

    // Both the long front term and the badge render.
    const frontEl = screen.getByText(longFront);
    expect(frontEl).toBeInTheDocument();
    expect(screen.getByLabelText("CEFR level C1")).toBeInTheDocument();

    // The centered content block is the parent of the front <p>; pin the
    // reserved horizontal-padding class on it.
    const contentBlock = frontEl.parentElement;
    expect(contentBlock).not.toBeNull();
    expect(contentBlock).toHaveClass("px-10");

    // The outer card container must carry `relative`: the absolutely-positioned
    // badge anchors to it, so removing `relative` would dislocate the badge to
    // the viewport. Navigate up from the content block to the outer card <div>.
    const outerCard = contentBlock?.parentElement;
    expect(outerCard).not.toBeNull();
    expect(outerCard).toHaveClass("relative");
  });
});
