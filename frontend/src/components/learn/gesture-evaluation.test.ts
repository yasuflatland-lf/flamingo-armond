import { describe, expect, it } from "vitest";
import {
  DIRECTION_LOCK_PX,
  evaluateSwipeGesture,
  HORIZONTAL_COMMIT_PX,
  HORIZONTAL_COMMIT_VX,
  HORIZONTAL_FLICK_MIN_PX,
} from "./gesture-evaluation";

const base = {
  active: true,
  mx: 0,
  my: 0,
  vx: 0,
  vy: 0,
  yDir: 0,
};

describe("evaluateSwipeGesture — direction locking (jitter suppression)", () => {
  it("does not lock a direction for sub-threshold horizontal movement (5px)", () => {
    const r = evaluateSwipeGesture({ ...base, mx: 5 });
    expect(r.direction).toBeNull();
    expect(r.progress).toBe(0);
  });

  it("does not lock a direction at the legacy 8px threshold", () => {
    const r = evaluateSwipeGesture({ ...base, mx: 10 });
    expect(r.direction).toBeNull();
    expect(r.progress).toBe(0);
  });

  it("locks 'right' once movement exceeds DIRECTION_LOCK_PX", () => {
    const r = evaluateSwipeGesture({ ...base, mx: DIRECTION_LOCK_PX + 2 });
    expect(r.direction).toBe("right");
    expect(r.progress).toBeGreaterThan(0);
  });

  it("locks 'left' for negative mx above the lock threshold", () => {
    const r = evaluateSwipeGesture({ ...base, mx: -(DIRECTION_LOCK_PX + 2) });
    expect(r.direction).toBe("left");
  });

  it("locks 'down' for positive my above the lock threshold", () => {
    const r = evaluateSwipeGesture({ ...base, my: DIRECTION_LOCK_PX + 2 });
    expect(r.direction).toBe("down");
  });

  it("does not lock 'down' for sub-threshold my", () => {
    const r = evaluateSwipeGesture({ ...base, my: 10 });
    expect(r.direction).toBeNull();
  });

  it("DIRECTION_LOCK_PX is at least 12 (jitter suppression)", () => {
    expect(DIRECTION_LOCK_PX).toBeGreaterThanOrEqual(12);
  });
});

describe("evaluateSwipeGesture — horizontal commit threshold (premature commit suppression)", () => {
  it("does NOT commit a horizontal swipe at 100px with low velocity", () => {
    const r = evaluateSwipeGesture({ ...base, active: false, mx: 100, vx: 0.2 });
    expect(r.shouldSwipe).toBe(false);
  });

  it("does NOT commit a horizontal swipe at 100px with the legacy velocity (0.5)", () => {
    // Previously vx > 0.45 was enough to commit at any distance — this is the
    // accidental-flick failure mode the user reported.
    const r = evaluateSwipeGesture({ ...base, active: false, mx: 100, vx: 0.5 });
    expect(r.shouldSwipe).toBe(false);
  });

  it("does NOT commit a horizontal swipe at the legacy 120px distance threshold", () => {
    const r = evaluateSwipeGesture({ ...base, active: false, mx: 120, vx: 0 });
    expect(r.shouldSwipe).toBe(false);
  });

  it("commits a horizontal swipe at HORIZONTAL_COMMIT_PX", () => {
    const r = evaluateSwipeGesture({
      ...base,
      active: false,
      mx: HORIZONTAL_COMMIT_PX + 1,
      vx: 0,
    });
    expect(r.shouldSwipe).toBe(true);
    expect(r.direction).toBe("right");
  });

  it("commits a left swipe at -HORIZONTAL_COMMIT_PX", () => {
    const r = evaluateSwipeGesture({
      ...base,
      active: false,
      mx: -(HORIZONTAL_COMMIT_PX + 1),
      vx: 0,
    });
    expect(r.shouldSwipe).toBe(true);
    expect(r.direction).toBe("left");
  });

  it("HORIZONTAL_COMMIT_PX is at least 150 (longer swipe required)", () => {
    expect(HORIZONTAL_COMMIT_PX).toBeGreaterThanOrEqual(150);
  });

  it("HORIZONTAL_COMMIT_VX is at least 0.8 (no accidental light-flick commits)", () => {
    expect(HORIZONTAL_COMMIT_VX).toBeGreaterThanOrEqual(0.8);
  });
});

describe("evaluateSwipeGesture — flick path (high velocity, mid distance)", () => {
  it("commits via flick when distance >= HORIZONTAL_FLICK_MIN_PX AND vx > HORIZONTAL_COMMIT_VX", () => {
    const r = evaluateSwipeGesture({
      ...base,
      active: false,
      mx: HORIZONTAL_FLICK_MIN_PX + 5,
      vx: HORIZONTAL_COMMIT_VX + 0.1,
    });
    expect(r.shouldSwipe).toBe(true);
  });

  it("does NOT commit via flick when distance is below HORIZONTAL_FLICK_MIN_PX even at very high velocity", () => {
    // A 20px move with a 2.0 px/ms velocity is the "twitch" case — almost no
    // travel, just a fast finger jiggle. Must not commit.
    const r = evaluateSwipeGesture({
      ...base,
      active: false,
      mx: HORIZONTAL_FLICK_MIN_PX - 10,
      vx: 2.0,
    });
    expect(r.shouldSwipe).toBe(false);
  });

  it("HORIZONTAL_FLICK_MIN_PX is at least 30 (no twitch commits)", () => {
    expect(HORIZONTAL_FLICK_MIN_PX).toBeGreaterThanOrEqual(30);
  });
});

describe("evaluateSwipeGesture — vertical (down) commit unchanged", () => {
  it("commits a down swipe at the legacy 96px distance threshold", () => {
    const r = evaluateSwipeGesture({ ...base, active: false, my: 100, vy: 0 });
    expect(r.shouldSwipe).toBe(true);
    expect(r.direction).toBe("down");
  });

  it("commits a down swipe via the velocity path", () => {
    const r = evaluateSwipeGesture({ ...base, active: false, my: 60, vy: 0.5 });
    expect(r.shouldSwipe).toBe(true);
    expect(r.direction).toBe("down");
  });

  it("does not commit a down swipe with insufficient motion", () => {
    const r = evaluateSwipeGesture({ ...base, active: false, my: 30, vy: 0.1 });
    expect(r.shouldSwipe).toBe(false);
  });
});

describe("evaluateSwipeGesture — shouldSwipe is false while active (mid-drag)", () => {
  it("never commits while the gesture is still active, even past commit distance", () => {
    const r = evaluateSwipeGesture({
      ...base,
      active: true,
      mx: HORIZONTAL_COMMIT_PX + 100,
      vx: 5,
    });
    expect(r.shouldSwipe).toBe(false);
    // …but direction/progress are still reported so the overlay paints.
    expect(r.direction).toBe("right");
    expect(r.progress).toBeGreaterThan(0);
  });
});
