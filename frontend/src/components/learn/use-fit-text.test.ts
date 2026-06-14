import { describe, expect, it } from "vitest";
import { computeFitFontSize, solveFitBySearch } from "./use-fit-text";

describe("computeFitFontSize", () => {
  const BOUNDS = { maxPx: 48, minPx: 18 };

  it("returns the max size when the word already fits within the available width", () => {
    expect(computeFitFontSize({ availableWidth: 300, intrinsicWidth: 200, ...BOUNDS })).toBe(48);
  });

  it("treats an exact fit as fitting (no shrink at the boundary)", () => {
    expect(computeFitFontSize({ availableWidth: 200, intrinsicWidth: 200, ...BOUNDS })).toBe(48);
  });

  it("shrinks proportionally when the word overflows the available width", () => {
    // 48 * 100 / 200 = 24
    expect(computeFitFontSize({ availableWidth: 100, intrinsicWidth: 200, ...BOUNDS })).toBe(24);
  });

  it("floors the shrunk size to a whole pixel", () => {
    // 48 * 100 / 150 = 32 → fits; use a ratio that is non-integer:
    // 48 * 100 / 140 = 34.28… → floor 34
    expect(computeFitFontSize({ availableWidth: 100, intrinsicWidth: 140, ...BOUNDS })).toBe(34);
  });

  it("clamps to the minimum size for a pathologically long word", () => {
    // 48 * 10 / 400 = 1.2 → floor 1 → clamped up to minPx (18)
    expect(computeFitFontSize({ availableWidth: 10, intrinsicWidth: 400, ...BOUNDS })).toBe(18);
  });

  it("returns the max size when the container width is unmeasured (jsdom / pre-layout)", () => {
    expect(computeFitFontSize({ availableWidth: 0, intrinsicWidth: 0, ...BOUNDS })).toBe(48);
    expect(computeFitFontSize({ availableWidth: 0, intrinsicWidth: 200, ...BOUNDS })).toBe(48);
  });

  it("returns the max size when the text width is unmeasured (empty / hidden element)", () => {
    expect(computeFitFontSize({ availableWidth: 300, intrinsicWidth: 0, ...BOUNDS })).toBe(48);
  });
});

describe("solveFitBySearch", () => {
  it("returns maxPx when the largest size already fits", () => {
    expect(solveFitBySearch(() => true, 14, 48)).toBe(48);
  });

  it("returns minPx when even the smallest size does not fit", () => {
    expect(solveFitBySearch(() => false, 14, 48)).toBe(14);
  });

  it("finds the largest fitting size for a monotone threshold predicate", () => {
    // fits(px) is true iff px <= 30 → the largest fitting integer size is 30.
    expect(solveFitBySearch((px) => px <= 30, 14, 48)).toBe(30);
  });

  it("floors a fractional maxPx before searching", () => {
    expect(solveFitBySearch(() => true, 14, 48.9)).toBe(48);
  });
});
