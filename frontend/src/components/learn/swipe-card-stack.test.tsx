// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SwipeCardData } from "./swipe-card";
import type { SwipeCardStackHandle } from "./swipe-card-stack";
import { SwipeCardStack } from "./swipe-card-stack";

// SwipeCard loads AnimatedCard via next/dynamic (ssr: false).
// In jsdom the dynamic import always resolves to the loading fallback (null),
// so we replace the whole module with a thin stub that exposes the same props
// and lets tests call onSwipeProgress / onSwipe directly via data-testid.
vi.mock("./swipe-card", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./swipe-card")>();
  return {
    ...actual,
    SwipeCard: ({
      card,
      onSwipe,
      onSwipeProgress,
    }: {
      card: SwipeCardData;
      isActive: boolean;
      onSwipe: (card: SwipeCardData, direction: "left" | "down" | "right") => void;
      onSwipeProgress?: (direction: "left" | "down" | "right" | null, progress: number) => void;
    }) => (
      <div
        data-testid="swipe-card-stub"
        data-card-id={card.id}
        // Attach helpers so tests can simulate gesture events.
        onPointerDown={() => onSwipeProgress?.("right", 0.5)}
        onPointerUp={() => {
          onSwipeProgress?.(null, 0);
          onSwipe(card, "right");
        }}
      >
        {card.front}
      </div>
    ),
  };
});

// Also mock useReducedMotion so tests can override it per-suite.
vi.mock("@/lib/use-reduced-motion", () => ({
  useReducedMotion: vi.fn(() => false),
}));

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// describe: Session-complete count line
// ---------------------------------------------------------------------------

describe("SwipeCardStack — Session-complete count line", () => {
  const baseProps = {
    cards: [] as Parameters<typeof SwipeCardStack>[0]["cards"],
    onCardSwiped: vi.fn(),
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

// ---------------------------------------------------------------------------
// describe: keydown listener stability (activeCardRef)
// ---------------------------------------------------------------------------

describe("SwipeCardStack — keydown listener stability (activeCardRef)", () => {
  it("registers the keydown listener exactly once even after activeCard changes via rerender", () => {
    const addSpy = vi.spyOn(window, "addEventListener");
    const removeSpy = vi.spyOn(window, "removeEventListener");

    const onCardSwiped = vi.fn();
    const { rerender, unmount } = render(
      <SwipeCardStack cards={[cardA, cardB]} onCardSwiped={onCardSwiped} />,
    );

    // Count only the "keydown" registrations from the initial render.
    const addedAfterMount = addSpy.mock.calls.filter(([type]) => type === "keydown").length;
    expect(addedAfterMount).toBe(1);

    // Simulate the parent advancing the deck: cardA was swiped, now cardB is first.
    rerender(<SwipeCardStack cards={[cardB]} onCardSwiped={onCardSwiped} />);

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
    vi.useFakeTimers({ shouldAdvanceTime: false });
    const onCardSwiped = vi.fn();
    const { rerender } = render(
      <SwipeCardStack cards={[cardA, cardB]} onCardSwiped={onCardSwiped} />,
    );

    // First ArrowLeft — activeCard is cardA. triggerSwipe sets a 180ms timeout.
    fireEvent.keyDown(document, { key: "ArrowLeft" });
    vi.runAllTimers();
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenNthCalledWith(1, cardA, "left");

    // Simulate the parent advancing the deck after the first swipe.
    rerender(<SwipeCardStack cards={[cardB]} onCardSwiped={onCardSwiped} />);

    // Second ArrowLeft — activeCardRef must now point to cardB, not cardA.
    fireEvent.keyDown(document, { key: "ArrowLeft" });
    vi.runAllTimers();
    expect(onCardSwiped).toHaveBeenCalledTimes(2);
    expect(onCardSwiped).toHaveBeenNthCalledWith(2, cardB, "left");

    vi.useRealTimers();
  });
});

// ---------------------------------------------------------------------------
// describe: triggerSwipe via imperative ref
// ---------------------------------------------------------------------------

describe("SwipeCardStack — triggerSwipe via imperative ref", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: false });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("calls onCardSwiped with (activeCard, direction) after the commit delay", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    render(<SwipeCardStack cards={[cardA, cardB]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("right");
    });
    // Not fired yet — 180ms pending.
    expect(onCardSwiped).not.toHaveBeenCalled();

    act(() => {
      vi.runAllTimers();
    });
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "right");
  });

  it("calls onCardSwiped with 'right' direction via triggerSwipe", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    render(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("right");
    });
    act(() => {
      vi.runAllTimers();
    });

    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "right");
  });

  it("cancels the first pending commit when triggerSwipe is called again before it fires", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    render(<SwipeCardStack cards={[cardA, cardB]} onCardSwiped={onCardSwiped} ref={ref} />);

    // First programmatic swipe — schedules a 180ms commit.
    act(() => {
      ref.current?.triggerSwipe("right");
    });
    // Advance 100ms — first commit not yet fired.
    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(onCardSwiped).not.toHaveBeenCalled();

    // Second swipe before the first fires — should cancel the first timer and
    // schedule a new one for "left".
    act(() => {
      ref.current?.triggerSwipe("left");
    });
    // Advance another 180ms (280ms total from start).
    act(() => {
      vi.advanceTimersByTime(180);
    });

    // Only the second swipe should have committed, exactly once.
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "left");
  });

  it("does not call onCardSwiped when unmount races a pending triggerSwipe timer", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    const { unmount } = render(
      <SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />,
    );

    // Schedule a commit but do not advance past the delay yet.
    act(() => {
      ref.current?.triggerSwipe("right");
    });
    // Advance 100ms — commit is still pending.
    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(onCardSwiped).not.toHaveBeenCalled();

    // Unmount while the timer is still pending.
    act(() => {
      unmount();
    });

    // Advance well past the 180ms threshold — the timer must have been cleared.
    act(() => {
      vi.advanceTimersByTime(200);
    });

    expect(onCardSwiped).not.toHaveBeenCalled();
  });

  it("shows overlay at progress=1 immediately after triggerSwipe before commit fires", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    render(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("right");
    });

    // Overlay label "Easy" should be visible at full opacity before the timeout.
    expect(screen.getByText("Easy")).toBeInTheDocument();
    // Commit has NOT fired yet.
    expect(onCardSwiped).not.toHaveBeenCalled();

    act(() => {
      vi.runAllTimers();
    });
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
  });

  it("shows overlay with 'Again' label when triggerSwipe('left') is called", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    render(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("left");
    });

    expect(screen.getByText("Again")).toBeInTheDocument();
    expect(onCardSwiped).not.toHaveBeenCalled();

    act(() => {
      vi.runAllTimers();
    });
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "left");
  });

  it("is a no-op when there is no active card", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    render(<SwipeCardStack cards={[]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      // Must not throw even when the deck is empty.
      ref.current?.triggerSwipe("right");
    });
    act(() => {
      vi.runAllTimers();
    });

    expect(onCardSwiped).not.toHaveBeenCalled();
  });

  it("fires onCardSwiped after exactly 180ms in normal motion mode", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    render(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("down");
    });

    // 179ms — not yet committed.
    act(() => {
      vi.advanceTimersByTime(179);
    });
    expect(onCardSwiped).not.toHaveBeenCalled();

    // 1ms more — now committed.
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "down");
  });
});

// ---------------------------------------------------------------------------
// describe: gesture-driven overlay via SwipeCard callbacks
// ---------------------------------------------------------------------------

describe("SwipeCardStack — gesture-driven overlay via SwipeCard callbacks", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: false });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows the overlay when onSwipeProgress fires right/0.5 (pointer-down), hides it when progress resets", () => {
    const onCardSwiped = vi.fn();

    render(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} />);

    const stub = screen.getByTestId("swipe-card-stub");

    // Simulate drag start — the mock calls onSwipeProgress("right", 0.5).
    act(() => {
      fireEvent.pointerDown(stub);
    });

    // Overlay should show the "Easy" label at partial opacity.
    expect(screen.getByText("Easy")).toBeInTheDocument();

    // Simulate drag release — the mock calls onSwipeProgress(null, 0) then onSwipe.
    // onSwipe goes through handleGestureCommit which clears the overlay immediately.
    act(() => {
      fireEvent.pointerUp(stub);
    });

    // onSwipe commits synchronously via handleGestureCommit, so overlay is gone.
    expect(screen.queryByText("Easy")).not.toBeInTheDocument();
    // The gesture commit fires onCardSwiped immediately (no delay for gesture path).
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "right");
  });
});

// ---------------------------------------------------------------------------
// describe: triggerSwipe with reduced motion
// ---------------------------------------------------------------------------

describe("SwipeCardStack — triggerSwipe with reduced motion", () => {
  beforeEach(async () => {
    const mod = await import("@/lib/use-reduced-motion");
    vi.mocked(mod.useReducedMotion).mockReturnValue(true);
  });
  afterEach(async () => {
    const mod = await import("@/lib/use-reduced-motion");
    vi.mocked(mod.useReducedMotion).mockReturnValue(false);
    vi.useRealTimers();
  });

  it("fires onCardSwiped synchronously (0ms) when reduced motion is active", () => {
    vi.useFakeTimers({ shouldAdvanceTime: false });
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    render(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("left");
    });

    // With reduced motion, onCardSwiped fires immediately — no timer advance needed.
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "left");
  });
});

// ---------------------------------------------------------------------------
// describe: keyboard direction coverage
// ---------------------------------------------------------------------------

describe("SwipeCardStack — keyboard triggers all three directions", () => {
  it.each([
    ["ArrowLeft", "left"],
    ["ArrowRight", "right"],
    ["ArrowDown", "down"],
  ] as const)(
    "fires onCardSwiped with direction '%s' → '%s' when the key is pressed",
    (key, expectedDirection) => {
      vi.useFakeTimers({ shouldAdvanceTime: false });
      const onCardSwiped = vi.fn();

      render(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} />);

      fireEvent.keyDown(document, { key });
      act(() => {
        vi.runAllTimers();
      });

      expect(onCardSwiped).toHaveBeenCalledTimes(1);
      expect(onCardSwiped).toHaveBeenCalledWith(cardA, expectedDirection);
      vi.useRealTimers();
    },
  );
});

// ---------------------------------------------------------------------------
// describe: internal state reset after card change
// ---------------------------------------------------------------------------

describe("SwipeCardStack — overlay state resets on active card change", () => {
  it("clears swipeDirection and swipeProgress when active card advances", () => {
    vi.useFakeTimers({ shouldAdvanceTime: false });
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    const { rerender } = render(
      <SwipeCardStack cards={[cardA, cardB]} onCardSwiped={onCardSwiped} ref={ref} />,
    );

    // Trigger a swipe — overlay should show.
    act(() => {
      ref.current?.triggerSwipe("right");
    });
    expect(screen.getByText("Easy")).toBeInTheDocument();

    // Advance timers so onCardSwiped fires; parent then re-renders with cardB first.
    act(() => {
      vi.runAllTimers();
    });

    // Simulate parent removing the swiped card from the deck.
    act(() => {
      rerender(<SwipeCardStack cards={[cardB]} onCardSwiped={onCardSwiped} ref={ref} />);
    });

    // The overlay should have cleared — no direction label visible.
    expect(screen.queryByText("Easy")).not.toBeInTheDocument();
    expect(screen.queryByText("Again")).not.toBeInTheDocument();
    expect(screen.queryByText("Hard")).not.toBeInTheDocument();

    vi.useRealTimers();
  });
});
