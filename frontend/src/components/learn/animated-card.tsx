"use client";

import { animated, easings, useSpring, useSpringRef } from "@react-spring/web";
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

// The reveal flip rotates the card to the RIGHT — `rotateY` runs 0 → -180deg
// (negative: the right edge swings toward the viewer first; positive rotateY
// would swing left). The flip eases IN: `easeInQuart` holds the card nearly
// still for roughly the first half of FLIP_DURATION_MS, then accelerates sharply
// into a "snap" as the back face arrives. The controller's spring config (used
// for drag / fly-off) would do the opposite — fast start, slow settle — so the
// flip overrides it with a fixed duration + ease-in easing.
const FLIP_DEGREES = -180;
const FLIP_DURATION_MS = 500;

// Imperative handle the parent stack uses to fly the active card off-screen
// programmatically (rating buttons / arrow keys). Stable contract — shape is
// consumed directly by SwipeCardStack; changes require a matching update there.
export type AnimatedCardHandle = {
  flyOut: (direction: SwipeDirection) => void;
};

type Props = {
  card: SwipeCardData;
  isActive: boolean;
  revealed: boolean;
  onReveal: () => void;
  onSwipe: (card: SwipeCardData, direction: SwipeDirection) => void;
  onSwipeProgress?: (direction: SwipeDirection | null, progress: number) => void;
  // Passed as a NORMAL prop, not React's `ref`: next/dynamic (ssr: false) wraps
  // this component with its own forwardRef and consumes React's `ref` for its
  // retry handle — it does not forward `ref` to the loaded component, only plain
  // props. A handle delivered via `ref` would never reach this useImperativeHandle.
  handleRef?: RefObject<AnimatedCardHandle | null>;
};

export function AnimatedCard({
  card,
  isActive,
  revealed,
  onReveal,
  onSwipe,
  onSwipeProgress,
  handleRef,
}: Props) {
  const reducedMotion = useReducedMotion();

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
  const [{ x, y, rotate, rotateY, scale }] = useSpring(() => ({
    x: 0,
    y: 0,
    rotate: 0,
    rotateY: revealed && !reducedMotion ? FLIP_DEGREES : 0,
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
  const reducedMotionRef = useRef(reducedMotion);
  useEffect(() => {
    reducedMotionRef.current = reducedMotion;
    void api.start({
      rotateY: revealed && !reducedMotion ? FLIP_DEGREES : 0,
      immediate: reducedMotion,
      // Ease-in flip: slow start, sharp "snap" finish. `immediate` (reduced
      // motion) skips the animation entirely, so the easing is a no-op there.
      config: { duration: FLIP_DURATION_MS, easing: easings.easeInQuart },
    });
  }, [api, reducedMotion, revealed]);

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
      // Rating no longer requires revealing first: the learner may rate a card
      // (rating buttons / arrow keys) while it is still front-only. Tapping to
      // reveal stays available as an optional way to check the answer.
      runExit(direction);
    },
    [runExit],
  );

  useImperativeHandle(handleRef, () => ({ flyOut }), [flyOut]);

  const revealCard = useCallback(() => {
    if (!isActive || revealed || exitingRef.current) return;
    onReveal();
  }, [isActive, onReveal, revealed]);

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
      // Swiping rates the card whether or not it is revealed — the learner can
      // swipe a front-only card directly, or reveal it first to check the answer.
      onSwipeProgress?.(active ? direction : null, active ? progress : 0);

      if (shouldSwipe && direction) {
        runExit(direction);
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
      onClick={revealCard}
      onKeyDown={(event) => {
        if (event.key !== " " && event.key !== "Enter") return;
        event.preventDefault();
        revealCard();
      }}
    >
      {reducedMotion ? (
        <CardContent card={card} revealed={revealed} />
      ) : (
        <animated.div
          className="relative h-full w-full [transform-style:preserve-3d]"
          data-testid="swipe-card-rotator"
          style={{
            transform: rotateY.to((value) => `rotateY(${value}deg)`),
          }}
        >
          <div className="absolute inset-0 [backface-visibility:hidden]">
            <CardContent card={card} revealed={false} />
          </div>
          {revealed && (
            <div className="absolute inset-0 [backface-visibility:hidden] [transform:rotateY(180deg)]">
              <CardContent card={card} revealed={true} />
            </div>
          )}
        </animated.div>
      )}
    </animated.article>
  );
}
