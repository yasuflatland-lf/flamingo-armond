// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { type RefObject, useImperativeHandle, useRef } from "react";
import { describe, expect, it, vi } from "vitest";
import type { SwipeCardData } from "@/components/learn/swipe-card";
import type { SwipeCardStackHandle } from "@/components/learn/swipe-card-stack";
import { SwipeSession } from "@/components/learn/swipe-session";

// ---------------------------------------------------------------------------
// Mock the heavy children so the test stays focused on SwipeSession's own shell
// wiring (the `disabled` flag derived from `cards.length` and the `topSlot`),
// without pulling in the next/dynamic AnimatedCard chunk or an intl provider.
// ---------------------------------------------------------------------------
type Direction = "left" | "down" | "right";

vi.mock("@/components/learn/swipe-card-stack", () => ({
  SwipeCardStack: (props: {
    cards: SwipeCardData[];
    onCardSwiped: (card: SwipeCardData, direction: Direction) => void;
    completedCount?: number;
    ref?: RefObject<SwipeCardStackHandle | null>;
  }) => {
    const activeCardRef = useRef(props.cards[0] ?? null);
    activeCardRef.current = props.cards[0] ?? null;
    useImperativeHandle(props.ref, () => ({
      triggerSwipe: (direction: Direction) => {
        const card = activeCardRef.current;
        if (card) props.onCardSwiped(card, direction);
      },
    }));
    return <div data-testid="swipe-card-stack">{props.cards.length} cards</div>;
  },
}));

vi.mock("@/components/learn/learn-action-bar", () => ({
  LearnActionBar: (props: { onRate: (d: Direction) => void; disabled: boolean }) => (
    <div data-testid="learn-action-bar" data-disabled={String(props.disabled)} />
  ),
}));

const CARD: SwipeCardData = {
  id: "c-1",
  front: "Hello",
  back: "Hola",
  cefrLevel: null,
  userCardState: { due: "2026-04-30T00:00:00Z", state: 0 },
  cardgroupId: "cg-1",
};

describe("<SwipeSession>", () => {
  it("renders the topSlot content in the leading grid row", () => {
    render(
      <SwipeSession
        cards={[CARD]}
        displayMode="ALWAYS_VISIBLE"
        onCardSwiped={() => {}}
        topSlot={<div data-testid="top-slot">banner</div>}
      />,
    );

    expect(screen.getByTestId("top-slot")).toBeInTheDocument();
    expect(screen.getByTestId("swipe-card-stack")).toBeInTheDocument();
  });

  it("enables the action bar while cards remain", () => {
    render(<SwipeSession cards={[CARD]} displayMode="ALWAYS_VISIBLE" onCardSwiped={() => {}} />);

    expect(screen.getByTestId("learn-action-bar")).toHaveAttribute("data-disabled", "false");
  });

  it("disables the action bar when the card list is empty", () => {
    render(<SwipeSession cards={[]} displayMode="ALWAYS_VISIBLE" onCardSwiped={() => {}} />);

    expect(screen.getByTestId("learn-action-bar")).toHaveAttribute("data-disabled", "true");
  });
});
