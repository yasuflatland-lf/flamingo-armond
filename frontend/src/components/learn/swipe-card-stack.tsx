"use client";

import type { RefObject } from "react";
import { useCallback, useEffect, useImperativeHandle, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { useReducedMotion } from "@/lib/use-reduced-motion";
import { type AnimatedCardHandle, SwipeCard, type SwipeCardData } from "./swipe-card";
import { SwipeDirectionOverlay } from "./swipe-direction-overlay";
import type { SwipeDirection } from "./types";

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

  // Imperative handle of the active (index 0) card. triggerSwipe drives the
  // fly-off through this; AnimatedCard commits onSwipe when its spring settles.
  const activeCardHandleRef = useRef<AnimatedCardHandle | null>(null);

  // Per-card once-guard: the gesture path and the programmatic flyOut path both
  // funnel their committed onSwipe through commitCard. The guard keyed on the
  // card id prevents a gesture + rating-button double-commit on the same card.
  const exitingCardIdRef = useRef<string | null>(null);

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

  // Single commit funnel. Both the gesture path and the programmatic flyOut
  // path deliver their committed onSwipe here (the gesture at pointer-release,
  // the fly-off when AnimatedCard's spring settles). The once-guard keyed on
  // the card id makes a gesture + rating-button double-commit on the same card
  // collapse to a single onCardSwiped call.
  const commitCard = useCallback((card: SwipeCardData, direction: SwipeDirection) => {
    if (exitingCardIdRef.current === card.id) return;
    exitingCardIdRef.current = card.id;
    setSwipeDirection(null);
    setSwipeProgress(0);
    onCardSwipedRef.current(card as TCard, direction);
  }, []);

  // onSwipe sink passed to every SwipeCard. The gesture path reaches it on
  // pointer-release; the fly-off path reaches it when the spring settles.
  const handleGestureCommit = useCallback(
    (swipedCard: SwipeCardData, direction: SwipeDirection) => {
      commitCard(swipedCard, direction);
    },
    [commitCard],
  );

  const triggerSwipe = useCallback(
    (direction: SwipeDirection) => {
      const card = activeCardRef.current;
      if (!card) return;
      // Already flying off this card — ignore repeat triggers.
      if (exitingCardIdRef.current === card.id) return;

      // Reduced motion: skip the animation and commit instantly.
      if (reducedMotionRef.current) {
        commitCard(card, direction);
        return;
      }

      // Paint the rating label at full intensity while the card flies, then
      // drive the fly-off — AnimatedCard commits via onSwipe on spring rest.
      setSwipeDirection(direction);
      setSwipeProgress(1);
      activeCardHandleRef.current?.flyOut(direction);
    },
    [commitCard],
  );

  useImperativeHandle(ref, () => ({ triggerSwipe }), [triggerSwipe]);

  // Reset overlay state and the exiting guard whenever the active card changes
  // so a programmatic triggerSwipe paint does not leak into the next card.
  // biome-ignore lint/correctness/useExhaustiveDependencies: tracking only the id of the active card is intentional — full-object deps would reset on referential changes to the same card.
  useEffect(() => {
    setSwipeDirection(null);
    setSwipeProgress(0);
    exitingCardIdRef.current = null;
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
            handleRef={index === 0 ? activeCardHandleRef : undefined}
          />
        </div>
      ))}
      <SwipeDirectionOverlay direction={swipeDirection} intensity={swipeProgress} />
    </div>
  );
}
