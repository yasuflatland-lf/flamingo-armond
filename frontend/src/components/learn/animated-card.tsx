"use client";

import { animated, useSpring } from "@react-spring/web";
import { useDrag } from "@use-gesture/react";
import { useCallback } from "react";
import { evaluateSwipeGesture } from "./gesture-evaluation";
import type { SwipeCardData } from "./swipe-card";
import { CardContent } from "./swipe-card";
import type { SwipeDirection } from "./types";

// Minimum movement (px) before `@use-gesture` considers the pointer to be in a
// drag. Raised above the previous 4px to stop a steady-finger pointer-down
// from registering a drag — the old threshold combined with the immediate
// scale jump (1 → 1.02) made the card visibly jitter on touch.
const USE_GESTURE_THRESHOLD = 12;

type Props = {
  card: SwipeCardData;
  isActive: boolean;
  onSwipe: (card: SwipeCardData, direction: SwipeDirection) => void;
  onSwipeProgress?: (direction: SwipeDirection | null, progress: number) => void;
};

export function AnimatedCard({ card, isActive, onSwipe, onSwipeProgress }: Props) {
  const [{ x, y, rotate, scale }, api] = useSpring(() => ({
    x: 0,
    y: 0,
    rotate: 0,
    scale: 1,
    config: { tension: 520, friction: 38 },
  }));

  const completeSwipe = useCallback(
    (direction: SwipeDirection) => {
      // Per direction, fly the card off-screen along the axis the gesture
      // committed to. The remaining axis stays at 0 / no rotation.
      let flyX = 0;
      let flyY = 0;
      let flyRotate = 0;
      switch (direction) {
        case "left":
          flyX = -window.innerWidth;
          flyRotate = -16;
          break;
        case "right":
          flyX = window.innerWidth;
          flyRotate = 16;
          break;
        case "down":
          flyY = window.innerHeight;
          break;
      }
      api.start({ x: flyX, y: flyY, rotate: flyRotate, scale: 0.92 });
      onSwipe(card, direction);
    },
    [api, card, onSwipe],
  );

  const bind = useDrag(
    ({ active, movement: [mx, my], direction: [, yDir], velocity: [vx, vy] }) => {
      if (!isActive) return;

      const { direction, progress, shouldSwipe } = evaluateSwipeGesture({
        active,
        mx,
        my,
        vx,
        vy,
        yDir,
      });
      onSwipeProgress?.(active ? direction : null, active ? progress : 0);

      if (shouldSwipe && direction) {
        completeSwipe(direction);
        return;
      }

      // `immediate` is restricted to transform-axis keys (x/y/rotate) so the
      // card tracks the pointer with no spring lag, while `scale` always
      // animates through the spring — that prevents the 1 → 1.02 jump on
      // drag-start which used to look like the card was "twitching".
      api.start({
        x: active ? mx : 0,
        y: active ? Math.max(my, -40) : 0,
        rotate: active ? mx / 16 : 0,
        scale: active ? 1.02 : 1,
        immediate: (key) => active && key !== "scale",
      });
    },
    {
      axis: undefined,
      filterTaps: true,
      pointer: { capture: false },
      rubberband: true,
      threshold: USE_GESTURE_THRESHOLD,
    },
  );

  return (
    <animated.article
      {...bind()}
      className="absolute inset-0 touch-none select-none cursor-grab active:cursor-grabbing"
      aria-label={`Flashcard: ${card.front}`}
      data-testid="swipe-card"
      tabIndex={isActive ? 0 : -1}
      style={{
        x,
        y,
        rotate: rotate.to((value) => `${value}deg`),
        scale,
      }}
    >
      <CardContent card={card} />
    </animated.article>
  );
}
