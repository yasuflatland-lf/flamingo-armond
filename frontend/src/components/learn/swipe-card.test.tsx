// @vitest-environment happy-dom
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
    render(<CardContent card={CARD} revealed={true} />);

    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("Hola")).toBeInTheDocument();
  });

  it("hides the back until the card is revealed", () => {
    render(<CardContent card={CARD} revealed={false} />);

    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.queryByText("Hola")).not.toBeInTheDocument();
  });

  it("shows the tap-affordance hint only while the card is un-revealed", () => {
    const { rerender } = render(<CardContent card={CARD} revealed={false} />);

    const hint = screen.getByTestId("tap-hint");
    expect(hint).toBeInTheDocument();
    // Decorative-only: never exposed to assistive tech.
    expect(hint).toHaveAttribute("aria-hidden", "true");
    // The pulse runs only when motion is allowed; a static faint dot remains
    // for prefers-reduced-motion users so the hint never disappears.
    expect(hint).toHaveClass("motion-safe:animate-tap-pulse");
    expect(hint).toHaveClass("motion-reduce:opacity-40");

    rerender(<CardContent card={CARD} revealed={true} />);
    expect(screen.queryByTestId("tap-hint")).not.toBeInTheDocument();
  });

  it("does not render rating buttons inside the card", () => {
    render(<CardContent card={CARD} revealed={true} />);

    expect(screen.queryByRole("button", { name: "Again" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Hard" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Easy" })).not.toBeInTheDocument();
  });

  it("does not render the CEFR badge itself — AnimatedCard pins it outside the flip rotator", () => {
    // The badge was lifted OUT of CardContent so the reveal flip cannot
    // duplicate it across the front/back faces (see animated-card.tsx). The CEFR
    // level still rides on the card data, but CardContent renders only the
    // textual content; AnimatedCard overlays the badge once over the card.
    render(<CardContent card={{ ...CARD, cefrLevel: "B1" }} revealed={true} />);

    expect(screen.queryByLabelText(/CEFR level/)).toBeNull();
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("Hola")).toBeInTheDocument();
  });

  it("wraps the front term at word boundaries, never mid-word (break-normal, not break-words/nowrap)", () => {
    // A multi-word front wraps across lines at spaces; a single overflowing
    // word is shrunk by useFitText rather than broken mid-word. The structural
    // precondition is `break-normal` (word-break: normal; overflow-wrap:
    // normal) — NOT `break-words` (breaks mid-word) and NOT `whitespace-nowrap`
    // (forbids the multi-line wrap the user asked for). jsdom has no layout
    // engine, so the shrink math is unit-tested in use-fit-text.test.ts.
    render(<CardContent card={{ ...CARD, front: "cardiovascular" }} revealed={false} />);

    const frontEl = screen.getByText("cardiovascular");
    expect(frontEl).toHaveClass("break-normal");
    expect(frontEl).not.toHaveClass("break-words");
    expect(frontEl).not.toHaveClass("break-all");
    expect(frontEl).not.toHaveClass("whitespace-nowrap");
  });

  it("balances the front term across lines (text-balance) so wrapping avoids a lone-word line", () => {
    render(<CardContent card={{ ...CARD, front: "I'll have to beg off" }} revealed={false} />);

    const frontEl = screen.getByText("I'll have to beg off");
    expect(frontEl).toHaveClass("text-balance");
  });

  it("wraps the back translation at word boundaries too, never mid-word", () => {
    render(<CardContent card={{ ...CARD, back: "electroencephalographically" }} revealed={true} />);

    const backEl = screen.getByText("electroencephalographically");
    expect(backEl).toHaveClass("break-normal");
    expect(backEl).not.toHaveClass("break-words");
    expect(backEl).not.toHaveClass("break-all");
  });

  it("applies the fixed front-fit ceiling (maxPx=48) as an inline font size when unrevealed", () => {
    // jsdom reports clientWidth/scrollWidth = 0, so useFitText returns the
    // ceiling unchanged — letting us pin the bounds and the style wiring
    // without a layout engine.
    render(<CardContent card={{ ...CARD, front: "Hello" }} revealed={false} />);

    expect(screen.getByText("Hello")).toHaveStyle({ fontSize: "48px" });
  });

  it("keeps the same front-fit ceiling (48px) when revealed — the headword does not shrink on flip", () => {
    render(<CardContent card={{ ...CARD, front: "Hello" }} revealed={true} />);

    expect(screen.getByText("Hello")).toHaveStyle({ fontSize: "48px" });
  });

  it("keeps the front size constant across an in-place reveal (reveal must not resize the headword)", () => {
    // The reduced-motion path keeps ONE CardContent instance and flips
    // `revealed` false→true in place. The headword size must stay constant
    // (48px → 48px); revealing only adds the translation below it.
    const { rerender } = render(
      <CardContent card={{ ...CARD, front: "Hello" }} revealed={false} />,
    );
    expect(screen.getByText("Hello")).toHaveStyle({ fontSize: "48px" });

    rerender(<CardContent card={{ ...CARD, front: "Hello" }} revealed={true} />);
    expect(screen.getByText("Hello")).toHaveStyle({ fontSize: "48px" });
  });

  it("reserves horizontal padding so a long front term cannot slide under the badge", () => {
    // jsdom has no real layout engine, so the overlap is guarded structurally:
    // the centered content block must carry the horizontal-padding utility that
    // keeps a pathological single-token `front` clear of the top-right CEFR
    // badge (the badge itself is overlaid by AnimatedCard, not CardContent).
    const longFront = "Pneumonoultramicroscopicsilicovolcanoconiosis";
    render(<CardContent card={{ ...CARD, front: longFront, cefrLevel: "C1" }} revealed={true} />);

    const frontEl = screen.getByText(longFront);
    expect(frontEl).toBeInTheDocument();

    // The centered content block is the parent of the front <p>; pin the
    // reserved horizontal-padding class on it.
    const contentBlock = frontEl.parentElement;
    expect(contentBlock).not.toBeNull();
    expect(contentBlock).toHaveClass("px-10");

    // The outer card container must carry `relative`: the tap-hint dot anchors
    // to it, and the AnimatedCard badge overlay assumes the card is positioned.
    // Removing `relative` would dislocate both. Navigate up from the content
    // block to the outer card <div>.
    const outerCard = contentBlock?.parentElement;
    expect(outerCard).not.toBeNull();
    expect(outerCard).toHaveClass("relative");
  });
});
