// @vitest-environment jsdom
import { act, fireEvent, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import type { SwipeCardData } from "./swipe-card";
import type { SwipeCardStackHandle } from "./swipe-card-stack";
import { SwipeCardStack } from "./swipe-card-stack";

// Module-scoped queue that simulates the AnimatedCard spring reaching "rest".
// The real AnimatedCard defers its onSwipe commit until the fly-off spring
// settles; the stub mirrors that by queuing the commit and letting tests
// settle it deterministically — no fake-timer advancement needed for the
// programmatic commit path.
let pendingFlyOuts: Array<() => void> = [];
function settleFlyOuts() {
  const fns = pendingFlyOuts;
  pendingFlyOuts = [];
  for (const fn of fns) fn();
}

// When true, the SwipeCard stub does NOT attach an imperative handle —
// mirroring the real-world window where the next/dynamic (ssr: false)
// AnimatedCard chunk has not loaded yet, so activeCardHandleRef.current is
// null. The stack's triggerSwipe must then fall back to committing directly.
let suppressHandle = false;

beforeEach(() => {
  pendingFlyOuts = [];
  suppressHandle = false;
});

// SwipeCard loads AnimatedCard via next/dynamic (ssr: false).
// In jsdom the dynamic import always resolves to the loading fallback (null),
// so we replace the whole module with a thin stub that exposes the same props
// and lets tests call onSwipeProgress / onSwipe / the handleRef.flyOut handle.
vi.mock("./swipe-card", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./swipe-card")>();
  const { useEffect, useImperativeHandle, useRef } = await import("react");
  return {
    ...actual,
    SwipeCard: ({
      card,
      handleRef,
      onSwipe,
      onSwipeProgress,
    }: {
      card: SwipeCardData;
      isActive: boolean;
      handleRef?: React.RefObject<import("./swipe-card").AnimatedCardHandle | null>;
      onSwipe: (card: SwipeCardData, direction: "left" | "down" | "right") => void;
      onSwipeProgress?: (direction: "left" | "down" | "right" | null, progress: number) => void;
    }) => {
      const mounted = useRef(true);
      useEffect(
        () => () => {
          mounted.current = false;
        },
        [],
      );
      useImperativeHandle(
        handleRef,
        () =>
          // When the handle is suppressed, return null so the parent's
          // activeCardHandleRef.current stays null — the un-loaded-chunk case.
          // The cast keeps useImperativeHandle's factory type happy; the parent
          // tolerates a null current (it falls back to committing directly).
          (suppressHandle
            ? null
            : {
                // flyOut defers the commit to "rest" (settleFlyOuts), mirroring
                // the real AnimatedCard which commits onSwipe from the spring
                // settle, mount-guarded so a settle after unmount is a no-op.
                flyOut: (direction: "left" | "down" | "right") => {
                  pendingFlyOuts.push(() => {
                    if (mounted.current) onSwipe(card, direction);
                  });
                },
              }) as import("./swipe-card").AnimatedCardHandle,
        [card, onSwipe],
      );
      return (
        <div
          data-testid="swipe-card-stub"
          data-card-id={card.id}
          // Attach helpers so tests can simulate gesture events. pointerUp
          // calls onSwipe directly — that is the gesture reaching the stack.
          onPointerDown={() => onSwipeProgress?.("right", 0.5)}
          onPointerUp={() => {
            onSwipeProgress?.(null, 0);
            onSwipe(card, "right");
          }}
        >
          {card.front}
        </div>
      );
    },
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
  cefrLevel: null,
  userCardState: {
    due: "2026-01-01",
    state: 0,
  },
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
    renderWithIntl(<SwipeCardStack {...baseProps} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.queryByText(/You reviewed/)).not.toBeInTheDocument();
  });

  it("renders Session-complete heading and no count line when completedCount is 0", () => {
    renderWithIntl(<SwipeCardStack {...baseProps} completedCount={0} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.queryByText(/You reviewed/)).not.toBeInTheDocument();
  });

  it("renders singular count line when completedCount is 1", () => {
    renderWithIntl(<SwipeCardStack {...baseProps} completedCount={1} />);
    expect(screen.getByRole("heading", { name: "Session complete" })).toBeInTheDocument();
    expect(screen.getByText("You reviewed 1 card in this batch.")).toBeInTheDocument();
  });

  it("renders plural count line when completedCount is 2", () => {
    renderWithIntl(<SwipeCardStack {...baseProps} completedCount={2} />);
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
    const { rerender, unmount } = renderWithIntl(
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
    const onCardSwiped = vi.fn();
    const { rerender } = renderWithIntl(
      <SwipeCardStack cards={[cardA, cardB]} onCardSwiped={onCardSwiped} />,
    );

    // First ArrowLeft — activeCard is cardA. triggerSwipe drives the fly-off,
    // which commits when the spring settles.
    fireEvent.keyDown(document, { key: "ArrowLeft" });
    expect(onCardSwiped).not.toHaveBeenCalled();
    act(() => {
      settleFlyOuts();
    });
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenNthCalledWith(1, cardA, "left");

    // Simulate the parent advancing the deck after the first swipe.
    rerender(<SwipeCardStack cards={[cardB]} onCardSwiped={onCardSwiped} />);

    // Second ArrowLeft — activeCardRef must now point to cardB, not cardA.
    fireEvent.keyDown(document, { key: "ArrowLeft" });
    act(() => {
      settleFlyOuts();
    });
    expect(onCardSwiped).toHaveBeenCalledTimes(2);
    expect(onCardSwiped).toHaveBeenNthCalledWith(2, cardB, "left");
  });
});

// ---------------------------------------------------------------------------
// describe: triggerSwipe via imperative ref (fly-off model)
// ---------------------------------------------------------------------------

describe("SwipeCardStack — triggerSwipe via imperative ref", () => {
  it("commits on spring rest, not before: onCardSwiped fires once after settle", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    renderWithIntl(<SwipeCardStack cards={[cardA, cardB]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("right");
    });
    // Fly-off queued but not yet settled — no commit.
    expect(onCardSwiped).not.toHaveBeenCalled();

    act(() => {
      settleFlyOuts();
    });
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "right");
  });

  it("commits with the 'right' direction via triggerSwipe", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    renderWithIntl(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("right");
    });
    act(() => {
      settleFlyOuts();
    });

    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "right");
  });

  it("does not call onCardSwiped when unmount races a pending fly-off", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    const { unmount } = renderWithIntl(
      <SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />,
    );

    // Queue a fly-off but do not settle it yet.
    act(() => {
      ref.current?.triggerSwipe("right");
    });
    expect(onCardSwiped).not.toHaveBeenCalled();

    // Unmount while the fly-off is still pending.
    act(() => {
      unmount();
    });

    // Settling now must be a no-op — the stub's mount guard drops the commit.
    act(() => {
      settleFlyOuts();
    });
    expect(onCardSwiped).not.toHaveBeenCalled();
  });

  it("shows overlay at full intensity immediately after triggerSwipe, before the commit settles", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    renderWithIntl(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("right");
    });

    // Overlay label "Easy" should be visible at full intensity before settle.
    expect(screen.getByText("Easy")).toBeInTheDocument();
    // Commit has NOT fired yet.
    expect(onCardSwiped).not.toHaveBeenCalled();

    act(() => {
      settleFlyOuts();
    });
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
  });

  it("shows overlay with 'Again' label when triggerSwipe('left') is called", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    renderWithIntl(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("left");
    });

    expect(screen.getByText("Again")).toBeInTheDocument();
    expect(onCardSwiped).not.toHaveBeenCalled();

    act(() => {
      settleFlyOuts();
    });
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "left");
  });

  it("shows overlay with 'Hard' label when triggerSwipe('down') is called", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    renderWithIntl(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("down");
    });

    expect(screen.getByText("Hard")).toBeInTheDocument();
    expect(onCardSwiped).not.toHaveBeenCalled();

    act(() => {
      settleFlyOuts();
    });
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "down");
  });

  it("commits exactly once when a gesture and a programmatic flyOut hit the same card", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    renderWithIntl(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    const stub = screen.getByTestId("swipe-card-stub");

    // Programmatic trigger queues a fly-off (deferred to settle).
    act(() => {
      ref.current?.triggerSwipe("right");
    });
    // Gesture commit reaches the stack synchronously via pointerUp.
    act(() => {
      fireEvent.pointerUp(stub);
    });

    // The gesture has already committed once through commitCard's once-guard.
    expect(onCardSwiped).toHaveBeenCalledTimes(1);

    // Settling the queued fly-off must NOT commit again (same card id guard).
    act(() => {
      settleFlyOuts();
    });
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "right");
  });

  it("is a no-op when there is no active card", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    renderWithIntl(<SwipeCardStack cards={[]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      // Must not throw even when the deck is empty.
      ref.current?.triggerSwipe("right");
    });
    act(() => {
      settleFlyOuts();
    });

    expect(onCardSwiped).not.toHaveBeenCalled();
  });

  it("commits directly when the card handle is null (dynamic chunk not yet attached)", () => {
    // Regression: a rating button or arrow key pressed before the next/dynamic
    // AnimatedCard chunk has attached its handle must NOT be silently dropped.
    // triggerSwipe falls back to commitCard so the press always lands.
    suppressHandle = true;
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    renderWithIntl(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("right");
    });

    // No handle to defer through — the commit fires synchronously.
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "right");
    // No fly-off was queued.
    expect(pendingFlyOuts).toHaveLength(0);

    // Settling confirms nothing double-commits.
    act(() => {
      settleFlyOuts();
    });
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
  });
});

// ---------------------------------------------------------------------------
// describe: gesture-driven overlay via SwipeCard callbacks
// ---------------------------------------------------------------------------

describe("SwipeCardStack — gesture-driven overlay via SwipeCard callbacks", () => {
  it("shows the overlay when onSwipeProgress fires right/0.5 (pointer-down), hides it when progress resets", () => {
    const onCardSwiped = vi.fn();

    renderWithIntl(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} />);

    const stub = screen.getByTestId("swipe-card-stub");

    // Simulate drag start — the mock calls onSwipeProgress("right", 0.5).
    act(() => {
      fireEvent.pointerDown(stub);
    });

    // Overlay should show the "Easy" label at partial opacity.
    expect(screen.getByText("Easy")).toBeInTheDocument();

    // Simulate drag release — the mock calls onSwipeProgress(null, 0) then onSwipe.
    // onSwipe goes through handleGestureCommit → commitCard which clears the overlay.
    act(() => {
      fireEvent.pointerUp(stub);
    });

    // onSwipe commits synchronously via commitCard, so overlay is gone.
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
  });

  it("fires onCardSwiped synchronously and does not invoke flyOut when reduced motion is active", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    renderWithIntl(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} ref={ref} />);

    act(() => {
      ref.current?.triggerSwipe("left");
    });

    // With reduced motion, onCardSwiped fires immediately — no settle needed.
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, "left");
    // flyOut must NOT have been queued — the reduced-motion path bypasses it.
    expect(pendingFlyOuts).toHaveLength(0);

    // Settling confirms nothing else commits.
    act(() => {
      settleFlyOuts();
    });
    expect(onCardSwiped).toHaveBeenCalledTimes(1);
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
  ] as const)("commits onCardSwiped with direction '%s' → '%s' on spring rest after the key is pressed", (key, expectedDirection) => {
    const onCardSwiped = vi.fn();

    renderWithIntl(<SwipeCardStack cards={[cardA]} onCardSwiped={onCardSwiped} />);

    fireEvent.keyDown(document, { key });
    // Commit is deferred to the fly-off spring rest — not yet fired.
    expect(onCardSwiped).not.toHaveBeenCalled();
    act(() => {
      settleFlyOuts();
    });

    expect(onCardSwiped).toHaveBeenCalledTimes(1);
    expect(onCardSwiped).toHaveBeenCalledWith(cardA, expectedDirection);
  });
});

// ---------------------------------------------------------------------------
// describe: internal state reset after card change
// ---------------------------------------------------------------------------

describe("SwipeCardStack — overlay state resets on active card change", () => {
  it("clears swipeDirection and swipeProgress when active card advances", () => {
    const onCardSwiped = vi.fn();
    const ref = createRef<SwipeCardStackHandle | null>();

    const { rerender } = renderWithIntl(
      <SwipeCardStack cards={[cardA, cardB]} onCardSwiped={onCardSwiped} ref={ref} />,
    );

    // Trigger a swipe — overlay should show.
    act(() => {
      ref.current?.triggerSwipe("right");
    });
    expect(screen.getByText("Easy")).toBeInTheDocument();

    // Settle so onCardSwiped fires; parent then re-renders with cardB first.
    act(() => {
      settleFlyOuts();
    });

    // Simulate parent removing the swiped card from the deck.
    act(() => {
      rerender(<SwipeCardStack cards={[cardB]} onCardSwiped={onCardSwiped} ref={ref} />);
    });

    // The overlay should have cleared — no direction label visible.
    expect(screen.queryByText("Easy")).not.toBeInTheDocument();
    expect(screen.queryByText("Again")).not.toBeInTheDocument();
    expect(screen.queryByText("Hard")).not.toBeInTheDocument();
  });
});
