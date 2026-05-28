import { describe, expect, it } from "vitest";
import { LONG_TERM_MEMORY_DAYS, stabilityToPercent } from "./fsrs-stability";

describe("stabilityToPercent — threshold and cap", () => {
  it("returns 100 at the long-term threshold (21 days)", () => {
    expect(stabilityToPercent(LONG_TERM_MEMORY_DAYS)).toBe(100);
  });

  it("caps at 100 for stability above the threshold (50 days)", () => {
    expect(stabilityToPercent(50)).toBe(100);
  });
});

describe("stabilityToPercent — linear mapping", () => {
  it("maps a new card stability (2.5 days) to 12", () => {
    expect(stabilityToPercent(2.5)).toBe(12);
  });

  it("maps a mid-range stability (10.5 days) to 50", () => {
    expect(stabilityToPercent(10.5)).toBe(50);
  });
});

describe("stabilityToPercent — non-finite inputs", () => {
  it("returns 0 for NaN", () => {
    expect(stabilityToPercent(Number.NaN)).toBe(0);
  });

  it("returns 0 for Infinity", () => {
    expect(stabilityToPercent(Number.POSITIVE_INFINITY)).toBe(0);
  });

  it("returns 0 for -Infinity", () => {
    expect(stabilityToPercent(Number.NEGATIVE_INFINITY)).toBe(0);
  });
});

describe("stabilityToPercent — negative inputs", () => {
  it("returns 0 for negative stability (-5 days)", () => {
    expect(stabilityToPercent(-5)).toBe(0);
  });
});
