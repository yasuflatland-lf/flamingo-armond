"use client";

import { animated, useSpring } from "@react-spring/web";
import { useDrag } from "@use-gesture/react";
import { type RefObject, useCallback, useEffect, useImperativeHandle, useRef } from "react";
import { useReducedMotion } from "@/lib/use-reduced-motion";
import { evaluateSwipeGesture } from "./gesture-evaluation";
import type { SwipeCardData } from "./swipe-card";
import { CardContent } from "./swipe-card";
import type { SwipeDirection } from "./types";

// Minimum movement (px) before `@use-gesture` considers the pointer to be in a
// drag. Raised above the previous 4px to stop a steady-finger pointer-down
// from registering a drag — the old threshold combined with the immediate
// scale jump (1 → 1.02) made the card visibly jitter on touch.
const USE_GESTURE_THRESHOLD = 12;

// Fly-off settles in a fixed duration so the deferred commit fires promptly and
// deterministically. ~200 ms matches the snappy feel the programmatic path had
// before, while still leaving the spring enough time to paint frames.
const FLY_OFF_DURATION_MS = 200;

// Imperative handle the parent stack uses to fly the active card off-screen
// programmatically (rating buttons / arrow keys). FIXED contract — consumed by
// the stack component.
export type AnimatedCardHandle = {
  flyOut: (direction: SwipeDirection) => void;
};

type Props = {
  card: SwipeCardData;
  isActive: boolean;
  onSwipe: (card: SwipeCardData, direction: SwipeDirection) => void;
  onSwipeProgress?: (direction: SwipeDirection | null, progress: number) => void;
  // Passed as a NORMAL prop, not React's `ref`: next/dynamic (ssr: false) wraps
  // this component with its own forwardRef and consumes React's `ref` for its
  // retry handle — it does not forward `ref` to the loaded component, only plain
  // props. A handle delivered via `ref` would never reach this useImperativeHandle.
  handleRef?: RefObject<AnimatedCardHandle | null>;
};

export function AnimatedCard({ card, isActive, onSwipe, onSwipeProgress, handleRef }: Props) {
  const [{ x, y, rotate, scale }, api] = useSpring(() => ({
    x: 0,
    y: 0,
    rotate: 0,
    scale: 1,
    config: { tension: 520, friction: 38 },
  }));

  // Guards against committing the same card twice (a second gesture or flyOut
  // after the exit has already started) and against committing after unmount —
  // the commit is deferred to the spring's rest, by which point the component
  // may be gone.
  const exitingRef = useRef(false);
  const mountedRef = useRef(true);
  useEffect(() => {
    return () => {
      mountedRef.current = false;
    };
  }, []);

  // Read reduced motion through a ref so the exit routines stay referentially
  // stable across renders — otherwise the gesture/imperative wiring would churn
  // on every media-query change.
  const reducedMotion = useReducedMotion();
  const reducedMotionRef = useRef(reducedMotion);
  useEffect(() => {
    reducedMotionRef.current = reducedMotion;
  }, [reducedMotion]);

  const runExit = useCallback(
    (direction: SwipeDirection) => {
      if (exitingRef.current) return;
      exitingRef.current = true;

      // Reduced motion: preserve the historical instant-removal behavior by
      // skipping the fly-off and committing synchronously.
      if (reducedMotionRef.current) {
        onSwipe(card, direction);
        return;
      }

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

      // Defer the commit until the fly-off spring settles so the animation is
      // actually painted (the parent drops the card on commit, unmounting it).
      // A fixed-duration config makes the rest deterministic; `finished` is
      // false if the spring was interrupted, and the mount guard prevents a
      // commit after unmount. `api.start` returns one async result per driven
      // controller, so await them all and require every settle to be finished.
      void Promise.all(
        api.start({
          x: flyX,
          y: flyY,
          rotate: flyRotate,
          scale: 0.92,
          config: { duration: FLY_OFF_DURATION_MS },
        }),
      ).then((results) => {
        if (mountedRef.current && results.every((result) => result.finished)) {
          onSwipe(card, direction);
        }
      });
    },
    [api, card, onSwipe],
  );

  const flyOut = useCallback(
    (direction: SwipeDirection) => {
      runExit(direction);
    },
    [runExit],
  );

  useImperativeHandle(handleRef, () => ({ flyOut }), [flyOut]);

  const completeSwipe = useCallback(
    (direction: SwipeDirection) => {
      runExit(direction);
    },
    [runExit],
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
