"use client";

import type { RefObject } from "react";
import { useCallback, useEffect, useImperativeHandle, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { useReducedMotion } from "@/lib/use-reduced-motion";
import { SwipeCard, type SwipeCardData } from "./swipe-card";
import { SwipeDirectionOverlay } from "./swipe-direction-overlay";
import type { SwipeDirection } from "./types";

/**
 * Delay between painting the swipe-progress overlay at full intensity and
 * firing onCardSwiped for programmatic (rating button / keyboard) commits.
 * Matches the previous parent-driven cadence so the overlay paints, then
 * the card flies out without the queue mutating mid-paint.
 */
const PROGRAMMATIC_COMMIT_DELAY_MS = 180;

/**
 * Imperative handle exposed by SwipeCardStack so parents can trigger a swipe
 * programmatically (e.g. from rating buttons) without threading the
 * direction through props. A stable handle + ref keeps the parent free of
 * re-renders driven by per-frame drag-progress updates from useDrag.
 */
export type SwipeCardStackHandle = {
  triggerSwipe: (direction: SwipeDirection) => void;
};

type Props<TCard extends SwipeCardData> = {
  cards: TCard[];
  onCardSwiped: (card: TCard, direction: SwipeDirection) => void;
  completedCount?: number;
  /**
   * Imperative handle ref. React 19 supports refs as plain props, so we
   * accept it directly instead of going through `forwardRef` — that pattern
   * erases the generic `TCard` parameter and breaks call-site inference.
   */
  ref?: RefObject<SwipeCardStackHandle | null>;
};

export function SwipeCardStack<TCard extends SwipeCardData>({
  cards,
  onCardSwiped,
  completedCount,
  ref,
}: Props<TCard>) {
  const activeCard = cards[0];

  // Drag-progress state ownership stays inside the stack, so the parent
  // component never re-renders during a gesture — per-frame onSwipeProgress
  // updates from useDrag would otherwise cascade a setState through
  // LearnClient's whole subtree on every pointer move.
  const [swipeDirection, setSwipeDirection] = useState<SwipeDirection | null>(null);
  const [swipeProgress, setSwipeProgress] = useState(0);
  const reducedMotion = useReducedMotion();
  const reducedMotionRef = useRef(reducedMotion);
  useEffect(() => {
    reducedMotionRef.current = reducedMotion;
  }, [reducedMotion]);

  // Mirror activeCard into a ref so triggerSwipe and the keydown listener do
  // not need to re-register on every card change — the ref is always current.
  const activeCardRef = useRef(activeCard);
  useEffect(() => {
    activeCardRef.current = activeCard;
  }, [activeCard]);

  // Track any pending programmatic commit so unmount or a new triggerSwipe
  // call can cancel it cleanly — avoids firing onCardSwiped after unmount.
  const pendingCommitRef = useRef<number | null>(null);
  useEffect(() => {
    return () => {
      if (pendingCommitRef.current != null) {
        window.clearTimeout(pendingCommitRef.current);
        pendingCommitRef.current = null;
      }
    };
  }, []);

  // Use a ref for the commit callback so the imperative handle stays stable
  // even when the parent passes a freshly-created onCardSwiped each render.
  const onCardSwipedRef = useRef(onCardSwiped);
  useEffect(() => {
    onCardSwipedRef.current = onCardSwiped;
  }, [onCardSwiped]);

  const handleSwipeProgress = useCallback((direction: SwipeDirection | null, progress: number) => {
    setSwipeDirection(direction);
    setSwipeProgress(progress);
  }, []);

  // Gesture-completed swipes take precedence over any pending programmatic
  // commit on the same card. Without this, a user who taps a rating button
  // and then flicks the card before the PROGRAMMATIC_COMMIT_DELAY_MS timer
  // fires would see onCardSwiped called twice for the same card — once via
  // the gesture path, once via the late-firing timer — producing a duplicate
  // mutation and an off-by-one completed counter. Clearing the timer here
  // makes the gesture path absorb the queued programmatic commit.
  const handleGestureCommit = useCallback(
    (swipedCard: SwipeCardData, direction: SwipeDirection) => {
      if (pendingCommitRef.current != null) {
        window.clearTimeout(pendingCommitRef.current);
        pendingCommitRef.current = null;
      }
      setSwipeDirection(null);
      setSwipeProgress(0);
      onCardSwipedRef.current(swipedCard as TCard, direction);
    },
    [],
  );

  const triggerSwipe = useCallback((direction: SwipeDirection) => {
    const card = activeCardRef.current;
    if (!card) return;

    // Paint the overlay at full progress so a programmatic swipe (rating
    // button / keyboard) shares the same visual affordance as a gesture
    // commit. With reduced motion, fire onCardSwiped immediately; otherwise
    // wait so the overlay paints before the queue mutates.
    setSwipeDirection(direction);
    setSwipeProgress(1);

    if (pendingCommitRef.current != null) {
      window.clearTimeout(pendingCommitRef.current);
      pendingCommitRef.current = null;
    }

    if (reducedMotionRef.current) {
      onCardSwipedRef.current(card, direction);
      return;
    }

    // Capture the card at trigger time so rapid clicks during the delay
    // window cannot route the commit at a card the user did not target.
    // The active-card id guard is a belt-and-suspenders defence on top of
    // handleGestureCommit clearing this timer when a gesture wins the race.
    pendingCommitRef.current = window.setTimeout(() => {
      pendingCommitRef.current = null;
      if (activeCardRef.current?.id !== card.id) return;
      onCardSwipedRef.current(card, direction);
    }, PROGRAMMATIC_COMMIT_DELAY_MS);
  }, []);

  useImperativeHandle(ref, () => ({ triggerSwipe }), [triggerSwipe]);

  // Reset overlay state whenever the active card changes so a programmatic
  // triggerSwipe paint does not leak into the next card's gesture.
  // biome-ignore lint/correctness/useExhaustiveDependencies: tracking only the id of the active card is intentional — full-object deps would reset on referential changes to the same card.
  useEffect(() => {
    setSwipeDirection(null);
    setSwipeProgress(0);
  }, [activeCard?.id]);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (!activeCardRef.current) return;
      const activeElement = document.activeElement;
      const tagName = activeElement?.tagName;
      if (tagName === "INPUT" || tagName === "TEXTAREA" || tagName === "SELECT") return;

      const directionByKey: Partial<Record<string, SwipeDirection>> = {
        ArrowLeft: "left",
        ArrowDown: "down",
        ArrowRight: "right",
      };
      const direction = directionByKey[event.key];
      if (!direction) return;
      event.preventDefault();
      triggerSwipe(direction);
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [triggerSwipe]); // triggerSwipe is now stable across activeCard changes

  if (!activeCard) {
    return (
      <div className="flex w-full max-w-xl flex-col items-center rounded-lg border border-dashed border-border p-8 text-center">
        <h1 className="mb-2 text-xl font-semibold">Session complete</h1>
        <p className="mb-6 text-sm text-muted-foreground">
          There are no due cards left in this batch.
        </p>
        {completedCount != null && completedCount > 0 && (
          <p className="mb-6 text-sm text-muted-foreground">
            You reviewed {completedCount} {completedCount === 1 ? "card" : "cards"} in this batch.
          </p>
        )}
        <Button type="button" variant="outline" onClick={() => window.location.reload()}>
          Refresh cards
        </Button>
      </div>
    );
  }

  return (
    <div className="relative h-full max-h-[520px] w-full max-w-xl sm:max-h-[580px]">
      {cards.slice(0, 3).map((card, index) => (
        <div
          key={card.id}
          aria-hidden={index !== 0}
          className="absolute inset-0 transition-transform"
          style={{
            zIndex: 10 - index,
            transform: `translateY(${index * 10}px) scale(${1 - index * 0.035})`,
            opacity: index === 2 ? 0.72 : 1,
            pointerEvents: index === 0 ? "auto" : "none",
          }}
        >
          <SwipeCard
            card={card}
            isActive={index === 0}
            onSwipe={handleGestureCommit}
            onSwipeProgress={handleSwipeProgress}
          />
        </div>
      ))}
      <SwipeDirectionOverlay direction={swipeDirection} intensity={swipeProgress} />
    </div>
  );
}
