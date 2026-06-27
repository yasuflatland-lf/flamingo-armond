"use client";

import type { ReactNode } from "react";
import { useCallback, useRef } from "react";
import { LearnActionBar } from "@/components/learn/learn-action-bar";
import type { SwipeCardData } from "@/components/learn/swipe-card";
import type { SwipeCardStackHandle } from "@/components/learn/swipe-card-stack";
import { SwipeCardStack } from "@/components/learn/swipe-card-stack";
import type { LearnDisplayMode, SwipeDirection } from "@/components/learn/types";

/**
 * Tailwind class for the swipe-session outer grid: three rows — a leading
 * `auto` slot (banner / spacer), a `1fr` card area, and a trailing `auto`
 * action bar. Exported so the loading skeleton can mirror the exact same
 * wrapper and the layout never shifts when the streamed session replaces the
 * placeholder.
 */
export const SWIPE_SESSION_GRID_CLASS = "grid min-h-0 flex-1 grid-rows-[auto_1fr_auto] gap-3";

type Props<TCard extends SwipeCardData> = {
  cards: TCard[];
  displayMode: LearnDisplayMode;
  onCardSwiped: (card: TCard, direction: SwipeDirection) => void;
  /**
   * Fills the leading `auto` grid row (an error banner, the practice banner,
   * …). A placeholder spacer is rendered when omitted so the card stays in the
   * `1fr` row and the action bar in the trailing `auto` row — without it, grid
   * auto-flow would assign the action bar to the `1fr` row.
   */
  topSlot?: ReactNode;
  /** Forwarded to SwipeCardStack's session-complete summary. */
  completedCount?: number;
};

/**
 * Presentational shell shared by LearnClient and PracticeClient: the swipe-grid
 * layout, the card area, the SwipeCardStack, and the LearnActionBar — plus the
 * imperative ref glue (`swipeStackRef` + `handleRate`) that lets the action bar
 * drive a programmatic swipe.
 *
 * It owns NO queue/data policy. Each client keeps its own queue orchestration
 * (LearnClient: server-write + optimistic delete + prefetch; PracticeClient:
 * local requeue) and feeds this shell the current `cards`, a `displayMode`, an
 * `onCardSwiped` sink, and a `topSlot`.
 */
export function SwipeSession<TCard extends SwipeCardData>({
  cards,
  displayMode,
  onCardSwiped,
  topSlot,
  completedCount,
}: Props<TCard>) {
  const swipeStackRef = useRef<SwipeCardStackHandle | null>(null);

  // SwipeCardStack owns the overlay paint + commit-delay timing internally, so
  // handleRate only needs to forward the direction through the imperative
  // handle. Keeping handleRate stable across renders preserves React.memo
  // bailouts on LearnActionBar.
  const handleRate = useCallback((direction: SwipeDirection) => {
    swipeStackRef.current?.triggerSwipe(direction);
  }, []);

  return (
    <section className={SWIPE_SESSION_GRID_CLASS}>
      {topSlot ?? <div aria-hidden="true" />}

      <div className="relative flex min-h-0 items-center justify-center overflow-hidden">
        <SwipeCardStack
          ref={swipeStackRef}
          cards={cards}
          displayMode={displayMode}
          onCardSwiped={onCardSwiped}
          completedCount={completedCount}
        />
      </div>
      <LearnActionBar onRate={handleRate} disabled={cards.length === 0} />
    </section>
  );
}
