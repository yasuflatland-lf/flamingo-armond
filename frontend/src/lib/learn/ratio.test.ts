import { describe, expect, it } from "vitest";
import { equalsPercent, gridPercent, MAX, MIN, STEP, shareTenths, tenthsToPercent } from "./ratio";

/** Every ratio the helpers accept: 0 <= n <= d <= 100 (5150 pairs), a superset of what the backend stores. */
const ALL_RATIOS = Array.from({ length: 100 }, (_, i) => i + 1).flatMap((d) =>
  Array.from({ length: d + 1 }, (_, n) => ({ numerator: n, denominator: d })),
);

/** Round-half-up of 1000n/d from the integer quotient and remainder. */
function exactTenths(n: number, d: number): number {
  const remainder = (1000 * n) % d;
  const quotient = (1000 * n - remainder) / d;
  return 2 * remainder >= d ? quotient + 1 : quotient;
}

/** Nearest 5% step by exact comparison of |100n/d - 5k| (ties to the larger k), then clamped. */
function exactGrid(n: number, d: number): number {
  let best = 0;
  for (let k = 1; k <= 20; k++) {
    if (Math.abs(100 * n - 5 * k * d) <= Math.abs(100 * n - 5 * best * d)) best = k;
  }
  return Math.min(MAX, Math.max(MIN, 5 * best));
}

describe("ratio constants", () => {
  it("pins the 5%-step control range", () => {
    expect(MIN).toBe(5);
    expect(MAX).toBe(80);
    expect(STEP).toBe(5);
  });
});

describe("shareTenths", () => {
  it("equals round-half-up of 1000n/d for every ratio with d <= 100 (exhaustive)", () => {
    for (const r of ALL_RATIOS) {
      expect(shareTenths(r), `${r.numerator}/${r.denominator}`).toBe(
        exactTenths(r.numerator, r.denominator),
      );
    }
  });

  it("rounds a repeating share to the nearest tenth in both directions", () => {
    // 1/3 = 33.333...% rounds down to 333; 2/3 = 66.666...% rounds up to 667.
    expect(shareTenths({ numerator: 1, denominator: 3 })).toBe(333);
    expect(shareTenths({ numerator: 2, denominator: 3 })).toBe(667);
  });

  it("rounds half up on an exact .x5 boundary", () => {
    // 1/16 = 6.25% and 3/16 = 18.75%: both are exactly halfway between tenths.
    expect(shareTenths({ numerator: 1, denominator: 16 })).toBe(63);
    expect(shareTenths({ numerator: 3, denominator: 16 })).toBe(188);
  });
});

describe("gridPercent", () => {
  it("is the nearest 5% step clamped to [MIN, MAX] for every ratio with d <= 100 (exhaustive)", () => {
    for (const r of ALL_RATIOS) {
      expect(gridPercent(r), `${r.numerator}/${r.denominator}`).toBe(
        exactGrid(r.numerator, r.denominator),
      );
    }
  });

  it("rounds half up when the share sits midway between two steps", () => {
    // 3/8 = 37.5%, exactly between the 35% and 40% steps.
    expect(gridPercent({ numerator: 3, denominator: 8 })).toBe(40);
  });

  it("clamps a share above the range down to MAX", () => {
    // 90% and 85% sat inside the old [5, 95] range but now exceed MAX = 80, so they
    // clamp; 99% rounds to grid index 20 (100%) and clamps too.
    expect(gridPercent({ numerator: 90, denominator: 100 })).toBe(80);
    expect(gridPercent({ numerator: 85, denominator: 100 })).toBe(80);
    expect(gridPercent({ numerator: 99, denominator: 100 })).toBe(MAX);
    expect(gridPercent({ numerator: 98, denominator: 100 })).toBe(MAX);
  });
});

describe("equalsPercent", () => {
  it("holds for gridPercent(r) exactly when r is an in-range grid step (exhaustive)", () => {
    for (const r of ALL_RATIOS) {
      const { numerator: n, denominator: d } = r;
      const onGrid = (100 * n) % (STEP * d) === 0 && 100 * n >= MIN * d && 100 * n <= MAX * d;
      expect(equalsPercent(r, gridPercent(r)), `${n}/${d}`).toBe(onGrid);
    }
  });

  it("is false for a fraction that only rounds to that percent", () => {
    // 1/3 displays as 33.3% but is not 33%.
    expect(equalsPercent({ numerator: 1, denominator: 3 }, 33)).toBe(false);
  });
});

describe("tenthsToPercent", () => {
  it("prints every tenths count back as t/10 and keeps complements summing to 100 (exhaustive)", () => {
    for (let t = 0; t <= 1000; t++) {
      expect(tenthsToPercent(t).toFixed(1)).toBe(`${Math.floor(t / 10)}.${t % 10}`);
      expect(tenthsToPercent(t) + tenthsToPercent(1000 - t)).toBe(100);
    }
  });
});
