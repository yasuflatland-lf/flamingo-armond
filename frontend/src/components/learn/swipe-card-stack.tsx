"use client";

import { useCallback, useEffect } from "react";
import type { SwipeDirection } from "@/app/learn/[cardgroupId]/learn-client";
import { Button } from "@/components/ui/button";
import { SwipeCard, type SwipeCardData } from "./swipe-card";
import { SwipeProgressOverlay } from "./swipe-progress-overlay";

type Props<TCard extends SwipeCardData> = {
  cards: TCard[];
  onCardSwiped: (card: TCard, direction: SwipeDirection) => void;
  onSwipeProgress?: (direction: SwipeDirection | null, progress: number) => void;
  swipeDirection: SwipeDirection | null;
  swipeProgress: number;
};

export function SwipeCardStack<TCard extends SwipeCardData>({
  cards,
  onCardSwiped,
  onSwipeProgress,
  swipeDirection,
  swipeProgress,
}: Props<TCard>) {
  const activeCard = cards[0];

  const triggerSwipe = useCallback(
    (direction: SwipeDirection) => {
      if (!activeCard) return;
      onCardSwiped(activeCard, direction);
    },
    [activeCard, onCardSwiped],
  );

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (!activeCard) return;
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
  }, [activeCard, triggerSwipe]);

  if (!activeCard) {
    return (
      <div className="flex w-full max-w-xl flex-col items-center rounded-lg border border-dashed border-border p-8 text-center">
        <h1 className="mb-2 text-xl font-semibold">Session complete</h1>
        <p className="mb-6 text-sm text-muted-foreground">
          There are no due cards left in this batch.
        </p>
        <Button type="button" variant="outline" onClick={() => window.location.reload()}>
          Refresh cards
        </Button>
      </div>
    );
  }

  return (
    <div className="relative h-[520px] w-full max-w-xl sm:h-[580px]">
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
            onSwipe={(swipedCard, direction) => onCardSwiped(swipedCard as TCard, direction)}
            onSwipeProgress={onSwipeProgress}
          />
        </div>
      ))}
      <SwipeProgressOverlay direction={swipeDirection} progress={swipeProgress} />
    </div>
  );
}
