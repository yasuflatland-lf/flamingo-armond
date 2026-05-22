import type { SwipeDirection } from "./types";

// Direction-lock threshold. Movement under this many pixels is treated as
// pre-drag jitter and reports no direction / zero progress so the overlay
// does not paint and `scale` does not jump on micro-movements.
export const DIRECTION_LOCK_PX = 14;

// A horizontal swipe commits when EITHER:
//   - travel distance exceeds HORIZONTAL_COMMIT_PX (slow but committed swipe), or
//   - travel >= HORIZONTAL_FLICK_MIN_PX AND velocity > HORIZONTAL_COMMIT_VX (flick).
// The flick minimum distance prevents a fast finger twitch (~20px at high
// velocity) from registering as an intentional swipe.
export const HORIZONTAL_COMMIT_PX = 160;
export const HORIZONTAL_COMMIT_VX = 0.9;
export const HORIZONTAL_FLICK_MIN_PX = 40;

// Vertical (down) commit thresholds — unchanged from the original tuning.
// "Hard" / unsure-of-answer gesture stays forgiving on purpose.
export const DOWN_COMMIT_PY = 96;
export const DOWN_COMMIT_VY = 0.35;
export const DOWN_COMMIT_YDIR = 0.9;

export type GestureInput = {
  active: boolean;
  mx: number;
  my: number;
  vx: number;
  vy: number;
  yDir: number;
};

export type GestureEvaluation = {
  direction: SwipeDirection | null;
  progress: number;
  shouldSwipe: boolean;
};

export function evaluateSwipeGesture({
  active,
  mx,
  my,
  vx,
  vy,
  yDir,
}: GestureInput): GestureEvaluation {
  const xAbs = Math.abs(mx);
  const yAbs = Math.abs(my);

  let direction: SwipeDirection | null = null;
  let progress = 0;

  if (xAbs >= yAbs && xAbs > DIRECTION_LOCK_PX) {
    direction = mx < 0 ? "left" : "right";
    progress = Math.min(xAbs / HORIZONTAL_COMMIT_PX, 1);
  } else if (my > DIRECTION_LOCK_PX) {
    direction = "down";
    progress = Math.min(yAbs / DOWN_COMMIT_PY, 1);
  }

  const shouldSwipe =
    !active &&
    direction !== null &&
    ((direction === "down" &&
      (yAbs > DOWN_COMMIT_PY || vy > DOWN_COMMIT_VY || yDir > DOWN_COMMIT_YDIR)) ||
      (direction !== "down" &&
        (xAbs > HORIZONTAL_COMMIT_PX ||
          (xAbs > HORIZONTAL_FLICK_MIN_PX && vx > HORIZONTAL_COMMIT_VX))));

  return { direction, progress, shouldSwipe };
}
