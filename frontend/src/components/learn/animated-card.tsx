"use client";

import { animated, useSpring, useSpringRef } from "@react-spring/web";
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
  // Drive the spring through an explicit `useSpringRef` rather than the api
  // returned by `useSpring(() => …)`. With the bare function form, react-spring
  // leaves the controller's internal `ctrl.ref` unset, so its per-commit layout
  // effect re-applies the initializer props (`x: 0`, default config) on EVERY
  // render. That is a no-op while the card rests at centre, but when a render
  // lands mid-fly-off (the release frame calls `onSwipeProgress(null, 0)`, which
  // re-renders the parent stack) it re-targets the in-flight spring back to
  // `x: 0` with the default config — the card snaps to centre, the fly-off
  // resolves `finished: false`, the deferred commit is dropped, and `exitingRef`
  // stays latched so every later gesture bails. Attaching an explicit ref makes
  // that layout effect queue the initializer instead of starting it, leaving the
  // imperative fly-off the sole driver. See @react-spring/core useSprings.
  const api = useSpringRef();
  const [{ x, y, rotate, scale }] = useSpring(() => ({
    x: 0,
    y: 0,
    rotate: 0,
    scale: 1,
    config: { tension: 520, friction: 38 },
    ref: api,
  }));

  // Guards against committing the same card twice (a second gesture or flyOut
  // after the exit has already started) and against committing after unmount —
  // the commit is deferred to the spring's rest, by which point the component
  // may be gone.
  const exitingRef = useRef(false);
  const mountedRef = useRef(true);
  useEffect(() => {
    // Reset to true in the effect body so React StrictMode's deliberate
    // setup → cleanup → setup cycle (and genuine remounts) leave the flag
    // true for the live component. Without this reset the cleanup sets the
    // flag false and the second setup never restores it, so the deferred
    // fly-off commit is silently dropped.
    mountedRef.current = true;
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
          immediate: false,
          config: { duration: FLY_OFF_DURATION_MS },
        }),
      )
        .then((results) => {
          if (mountedRef.current && results.every((result) => result.finished)) {
            onSwipe(card, direction);
          }
        })
        .catch((error) => {
          // The fly-off spring can reject on a teardown race (a frozen
          // SpringValue mid-animation). The animation is cosmetic but the
          // commit is the load-bearing side effect — dropping it would strand
          // the card in the deck. Log for visibility, then commit anyway
          // (mount-guarded so a post-unmount commit is still skipped).
          console.warn("[learn] fly-off spring rejected; committing anyway:", error);
          if (mountedRef.current) {
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
      // Once the exit fly-off has begun the card is committed and inert to
      // further drag handling. Without this guard a re-grab (the finger never
      // lifts, so @use-gesture starts a fresh gesture during the ~200 ms
      // fly-off) falls through to the spring-reset `else` below and calls
      // `api.start({ x: 0 })`, which interrupts the in-flight fly-off: the card
      // animates back to centre AND the fly-off spring resolves `finished:
      // false`, so the deferred `onSwipe` commit is dropped and the card is
      // stranded in the deck. Bailing here keeps the fly-off uninterrupted.
      if (!isActive || exitingRef.current) return;

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
