"use client";

import { animated, useSpring } from "@react-spring/web";
import { useDrag } from "@use-gesture/react";
import { useCallback } from "react";
import type { SwipeCardData } from "./swipe-card";
import { CardContent } from "./swipe-card";
import type { SwipeDirection } from "./types";

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

      const xAbs = Math.abs(mx);
      const yAbs = Math.abs(my);
      let direction: SwipeDirection | null = null;
      let progress = 0;

      if (xAbs >= yAbs && xAbs > 8) {
        direction = mx < 0 ? "left" : "right";
        progress = Math.min(xAbs / 120, 1);
      } else if (my > 8) {
        direction = "down";
        progress = Math.min(yAbs / 96, 1);
      }
      onSwipeProgress?.(active ? direction : null, active ? progress : 0);

      const shouldSwipe =
        !active &&
        direction &&
        ((direction === "down" && (yAbs > 96 || vy > 0.35 || yDir > 0.9)) ||
          (direction !== "down" && (xAbs > 120 || vx > 0.45)));

      if (shouldSwipe && direction) {
        completeSwipe(direction);
        return;
      }

      api.start({
        x: active ? mx : 0,
        y: active ? Math.max(my, -40) : 0,
        rotate: active ? mx / 16 : 0,
        scale: active ? 1.02 : 1,
        immediate: active,
      });
    },
    {
      axis: undefined,
      filterTaps: true,
      pointer: { capture: false },
      rubberband: true,
      threshold: 4,
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
