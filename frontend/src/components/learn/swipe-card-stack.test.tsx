// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { SwipeCardData } from "./swipe-card";
import { SwipeCardStack } from "./swipe-card-stack";

describe("SwipeCardStack — Session-complete count line", () => {
  const baseProps = {
    cards: [] as Parameters<typeof SwipeCardStack>[0]["cards"],
    onCardSwiped: vi.fn(),
    onSwipeProgress: vi.fn(),
    swipeDirection: null,
    swipeProgress: 0,
  };

  it("renders Session-complete heading and no count line when completedCount is undefined", () => {
    render(<SwipeCardStack {...baseProps} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.queryByText(/You reviewed/)).not.toBeInTheDocument();
  });

  it("renders Session-complete heading and no count line when completedCount is 0", () => {
    render(<SwipeCardStack {...baseProps} completedCount={0} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.queryByText(/You reviewed/)).not.toBeInTheDocument();
  });

  it("renders singular count line when completedCount is 1", () => {
    render(<SwipeCardStack {...baseProps} completedCount={1} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.getByText("You reviewed 1 card in this batch.")).toBeInTheDocument();
  });

  it("renders plural count line when completedCount is 2", () => {
    render(<SwipeCardStack {...baseProps} completedCount={2} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.getByText("You reviewed 2 cards in this batch.")).toBeInTheDocument();
  });
});

describe("SwipeCardStack — keydown listener stability (activeCardRef)", () => {
  const makeCard = (id: string): SwipeCardData => ({
    id,
    front: `front-${id}`,
    back: `back-${id}`,
    due: "2026-01-01",
    state: 0,
    cardgroupId: "cg-1",
  });

  const cardA = makeCard("card-a");
  const cardB = makeCard("card-b");

  it("registers the keydown listener exactly once even after activeCard changes via rerender", () => {
    const addSpy = vi.spyOn(window, "addEventListener");
    const removeSpy = vi.spyOn(window, "removeEventListener");

    const onCardSwiped = vi.fn();
    const { rerender, unmount } = render(
      <SwipeCardStack
        cards={[cardA, cardB]}
        onCardSwiped={onCardSwiped}
        swipeDirection={null}
        swipeProgress={0}
      />,
    );

    // Count only the "keydown" registrations from the initial render.
    const addedAfterMount = addSpy.mock.calls.filter(([type]) => type === "keydown").length;
    expect(addedAfterMount).toBe(1);

    // Simulate the parent advancing the deck: cardA was swiped, now cardB is first.
    rerender(
      <SwipeCardStack
        cards={[cardB]}
        onCardSwiped={onCardSwiped}
        swipeDirection={null}
        swipeProgress={0}
      />,
    );

    // The listener must NOT have been removed and re-added (triggerSwipe is stable
    // across activeCard changes because activeCardRef is used inside it).
    const addedAfterRerender = addSpy.mock.calls.filter(([type]) => type === "keydown").length;
    const removedAfterRerender = removeSpy.mock.calls.filter(([type]) => type === "keydown").length;
    expect(addedAfterRerender).toBe(1); // still exactly one add total
    expect(removedAfterRerender).toBe(0); // no remove before unmount

    unmount();
    addSpy.mockRestore();
    removeSpy.mockRestore();
  });

  it("fires onCardSwiped with the correct card after activeCard advances", () => {
    const onCardSwiped = vi.fn();
    const { rerender } = render(
      <SwipeCardStack
        cards={[cardA, cardB]}
        onCardSwiped={onCardSwiped}
        swipeDirection={null}
        swipeProgress={0}
      />,
    );

    // First ArrowLeft — activeCard is cardA.
    fireEvent.keyDown(document, { key: "ArrowLeft" });
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenNthCalledWith(1, cardA, "left");

    // Simulate the parent advancing the deck after the first swipe.
    rerender(
      <SwipeCardStack
        cards={[cardB]}
        onCardSwiped={onCardSwiped}
        swipeDirection={null}
        swipeProgress={0}
      />,
    );

    // Second ArrowLeft — activeCardRef must now point to cardB, not cardA.
    fireEvent.keyDown(document, { key: "ArrowLeft" });
    expect(onCardSwiped).toHaveBeenCalledTimes(2);
    expect(onCardSwiped).toHaveBeenNthCalledWith(2, cardB, "left");
  });
});
