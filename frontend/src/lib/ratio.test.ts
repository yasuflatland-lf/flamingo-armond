import { describe, expect, it } from "vitest";
import {
  equalsPercent,
  gridPercent,
  isOnGrid,
  MAX,
  MIN,
  type Ratio,
  STEP,
  shareTenths,
  tenthsToPercent,
} from "./ratio";

describe("ratio constants", () => {
  it("pins the 5%-step control range", () => {
    expect(MIN).toBe(5);
    expect(MAX).toBe(95);
    expect(STEP).toBe(5);
  });
});

describe("shareTenths", () => {
  it("returns an exact tenths count for a share that lands on a whole percent", () => {
    expect(shareTenths({ numerator: 4, denominator: 5 })).toBe(800);
    expect(shareTenths({ numerator: 33, denominator: 100 })).toBe(330);
  });

  it("keeps the tenth for an off-grid share that needs one", () => {
    // 1/8 = 12.5% exactly, so the tenths count is integral without rounding.
    expect(shareTenths({ numerator: 1, denominator: 8 })).toBe(125);
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

  it("handles the smallest and largest storable shares", () => {
    expect(shareTenths({ numerator: 1, denominator: 100 })).toBe(10);
    expect(shareTenths({ numerator: 99, denominator: 100 })).toBe(990);
  });
});

describe("isOnGrid", () => {
  it("is true for a share that is a whole multiple of 5", () => {
    expect(isOnGrid({ numerator: 4, denominator: 5 })).toBe(true);
    expect(isOnGrid({ numerator: 1, denominator: 2 })).toBe(true);
    expect(isOnGrid({ numerator: 7, denominator: 20 })).toBe(true);
  });

  it("is true at both ends of the selectable range", () => {
    // 1/20 = 5% = MIN, 19/20 = 95% = MAX.
    expect(isOnGrid({ numerator: 1, denominator: 20 })).toBe(true);
    expect(isOnGrid({ numerator: 19, denominator: 20 })).toBe(true);
  });

  it("is false for a share that misses the grid", () => {
    expect(isOnGrid({ numerator: 33, denominator: 100 })).toBe(false);
    expect(isOnGrid({ numerator: 1, denominator: 3 })).toBe(false);
    expect(isOnGrid({ numerator: 1, denominator: 8 })).toBe(false);
  });

  it("is false one percentage point off a grid step", () => {
    // 79% and 81% straddle the 80% step.
    expect(isOnGrid({ numerator: 79, denominator: 100 })).toBe(false);
    expect(isOnGrid({ numerator: 81, denominator: 100 })).toBe(false);
  });
});

describe("gridPercent", () => {
  it("returns the share itself when it is already a grid step", () => {
    expect(gridPercent({ numerator: 4, denominator: 5 })).toBe(80);
    expect(gridPercent({ numerator: 1, denominator: 2 })).toBe(50);
  });

  it("snaps an off-grid share to the nearest step", () => {
    expect(gridPercent({ numerator: 33, denominator: 100 })).toBe(35);
    expect(gridPercent({ numerator: 1, denominator: 3 })).toBe(35);
    expect(gridPercent({ numerator: 32, denominator: 100 })).toBe(30);
  });

  it("rounds half up when the share sits midway between two steps", () => {
    // 3/8 = 37.5%, exactly between the 35% and 40% steps.
    expect(gridPercent({ numerator: 3, denominator: 8 })).toBe(40);
  });

  it("clamps a share below the range up to MIN", () => {
    // 1% rounds to grid index 0 (0%), which the clamp lifts to MIN.
    expect(gridPercent({ numerator: 1, denominator: 100 })).toBe(MIN);
    expect(gridPercent({ numerator: 2, denominator: 100 })).toBe(MIN);
  });

  it("clamps a share above the range down to MAX", () => {
    // 99% rounds to grid index 20 (100%), which the clamp pulls back to MAX.
    expect(gridPercent({ numerator: 99, denominator: 100 })).toBe(MAX);
    expect(gridPercent({ numerator: 98, denominator: 100 })).toBe(MAX);
  });

  it("leaves the shares just inside the range unclamped", () => {
    // 3% rounds up to 5% and 97% rounds down to 95% on their own.
    expect(gridPercent({ numerator: 3, denominator: 100 })).toBe(5);
    expect(gridPercent({ numerator: 97, denominator: 100 })).toBe(95);
  });
});

describe("equalsPercent", () => {
  it("is true when the stored fraction is exactly that percent", () => {
    expect(equalsPercent({ numerator: 4, denominator: 5 }, 80)).toBe(true);
    expect(equalsPercent({ numerator: 33, denominator: 100 }, 33)).toBe(true);
  });

  it("is false for a fraction that only rounds to that percent", () => {
    // 1/3 displays as 33.3% but is not 33%.
    expect(equalsPercent({ numerator: 1, denominator: 3 }, 33)).toBe(false);
  });

  it("is false for a neighbouring percent", () => {
    expect(equalsPercent({ numerator: 4, denominator: 5 }, 85)).toBe(false);
    expect(equalsPercent({ numerator: 4, denominator: 5 }, 75)).toBe(false);
  });
});

describe("tenthsToPercent", () => {
  it("converts a whole-percent tenths count", () => {
    expect(tenthsToPercent(800)).toBe(80);
    expect(tenthsToPercent(0)).toBe(0);
    expect(tenthsToPercent(1000)).toBe(100);
  });

  it("keeps one decimal for an off-grid tenths count", () => {
    expect(tenthsToPercent(333)).toBe(33.3);
    expect(tenthsToPercent(63)).toBe(6.3);
  });

  it("prints back as the same one-decimal string", () => {
    expect(tenthsToPercent(333).toFixed(1)).toBe("33.3");
    expect(tenthsToPercent(667).toFixed(1)).toBe("66.7");
  });

  it("keeps a complement pair summing to exactly 100", () => {
    const ratio: Ratio = { numerator: 1, denominator: 3 };
    const tenths = shareTenths(ratio);
    expect(tenthsToPercent(tenths) + tenthsToPercent(1000 - tenths)).toBe(100);
  });
});
