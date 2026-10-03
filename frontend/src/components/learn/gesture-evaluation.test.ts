import fc from "fast-check";
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

/** Travel in px, biased onto every threshold edge. */
const px = fc.oneof(
  fc.integer({ min: -500, max: 500 }),
  fc.constantFrom(14, 15, 40, 41, 96, 97, 160, 161).chain((v) => fc.constantFrom(v, -v)),
);
const speed = fc.double({ min: 0, max: 3, noNaN: true });
const gesture = fc.record({
  active: fc.boolean(),
  mx: px,
  my: px,
  vx: speed,
  vy: speed,
  yDir: fc.double({ min: -1, max: 1, noNaN: true }),
});

describe("evaluateSwipeGesture — laws (property)", () => {
  it("reports no direction, zero progress and no commit inside the lock threshold", () => {
    const inside = fc.integer({ min: -DIRECTION_LOCK_PX, max: DIRECTION_LOCK_PX });
    fc.assert(
      fc.property(
        gesture,
        inside,
        fc.integer({ min: -500, max: DIRECTION_LOCK_PX }),
        (g, mx, my) => {
          const r = evaluateSwipeGesture({ ...g, mx, my });
          return r.direction === null && r.progress === 0 && !r.shouldSwipe;
        },
      ),
    );
  });

  it("keeps progress in [0, 1] and never commits while active or without a direction", () => {
    fc.assert(
      fc.property(gesture, (g) => {
        const r = evaluateSwipeGesture(g);
        const noCommit = g.active || r.direction === null;
        return r.progress >= 0 && r.progress <= 1 && !(noCommit && r.shouldSwipe);
      }),
    );
  });

  it("is mirror-symmetric: mx -> -mx swaps left and right with the same progress and commit", () => {
    const flip = { left: "right", right: "left", down: "down" } as const;
    fc.assert(
      fc.property(gesture, (g) => {
        const a = evaluateSwipeGesture(g);
        const b = evaluateSwipeGesture({ ...g, mx: -g.mx });
        return (
          (a.direction === null ? null : flip[a.direction]) === b.direction &&
          a.progress === b.progress &&
          a.shouldSwipe === b.shouldSwipe
        );
      }),
    );
  });

  it("commits a released horizontal swipe iff travel > COMMIT_PX or a flick (travel > FLICK_MIN_PX and vx > COMMIT_VX)", () => {
    fc.assert(
      fc.property(gesture, (g) => {
        const r = evaluateSwipeGesture({ ...g, active: false });
        if (r.direction !== "left" && r.direction !== "right") return true;
        const x = Math.abs(g.mx);
        const flick = x > HORIZONTAL_FLICK_MIN_PX && g.vx > HORIZONTAL_COMMIT_VX;
        return r.shouldSwipe === (x > HORIZONTAL_COMMIT_PX || flick);
      }),
    );
  });

  it("commits a released down swipe iff travel > 96px, vy > 0.35 or yDir > 0.9", () => {
    fc.assert(
      fc.property(gesture, (g) => {
        const r = evaluateSwipeGesture({ ...g, active: false });
        if (r.direction !== "down") return true;
        return r.shouldSwipe === (Math.abs(g.my) > 96 || g.vy > 0.35 || g.yDir > 0.9);
      }),
    );
  });
});

describe("evaluateSwipeGesture — direction locking (jitter suppression)", () => {
  it("locks 'right' once movement exceeds DIRECTION_LOCK_PX", () => {
    const r = evaluateSwipeGesture({ ...base, mx: DIRECTION_LOCK_PX + 2 });
    expect(r.direction).toBe("right");
    expect(r.progress).toBeGreaterThan(0);
  });

  it("locks 'down' for positive my above the lock threshold", () => {
    const r = evaluateSwipeGesture({ ...base, my: DIRECTION_LOCK_PX + 2 });
    expect(r.direction).toBe("down");
  });

  it("DIRECTION_LOCK_PX is at least 12 (jitter suppression)", () => {
    expect(DIRECTION_LOCK_PX).toBeGreaterThanOrEqual(12);
  });
});

describe("evaluateSwipeGesture — horizontal commit threshold (premature commit suppression)", () => {
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
});
