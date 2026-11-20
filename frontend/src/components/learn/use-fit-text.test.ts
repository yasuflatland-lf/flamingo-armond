import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { computeFitFontSize, solveFitBySearch } from "./use-fit-text";

describe("computeFitFontSize", () => {
  const BOUNDS = { maxPx: 48, minPx: 18 };

  it("stays in [minPx, maxPx], never grows as the text widens, and fits unless clamped (property)", () => {
    fc.assert(
      fc.property(
        fc.double({ min: 1, max: 800, noNaN: true }),
        fc.double({ min: 1, max: 3000, noNaN: true }),
        fc.double({ min: 1, max: 3000, noNaN: true }),
        fc.integer({ min: 8, max: 20 }),
        fc.integer({ min: 20, max: 64 }),
        (availableWidth, w1, w2, minPx, maxPx) => {
          const size = (w: number) =>
            computeFitFontSize({ availableWidth, intrinsicWidth: w, maxPx, minPx });
          const [narrow, wide] = w1 <= w2 ? [w1, w2] : [w2, w1];
          const s = size(wide);
          const fits = s === minPx || (s * wide) / maxPx <= availableWidth + 1e-9;
          return s >= minPx && s <= maxPx && size(narrow) >= s && fits;
        },
      ),
    );
  });

  it("treats an exact fit as fitting (no shrink at the boundary)", () => {
    expect(computeFitFontSize({ availableWidth: 200, intrinsicWidth: 200, ...BOUNDS })).toBe(48);
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
  it("returns the largest fitting integer clamped to [minPx, floor(maxPx)] for a threshold predicate (property)", () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 70 }),
        fc.integer({ min: 8, max: 30 }),
        fc.integer({ min: 30, max: 64 }),
        (k, minPx, maxPx) =>
          solveFitBySearch((px) => px <= k, minPx, maxPx) === Math.max(minPx, Math.min(maxPx, k)),
      ),
    );
  });

  it("floors a fractional maxPx before searching", () => {
    expect(solveFitBySearch(() => true, 14, 48.9)).toBe(48);
  });
});
