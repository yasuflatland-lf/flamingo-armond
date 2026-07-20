// Exact-rational arithmetic for the new-card ratio — the share of a learn
// session that is never-seen cards.
//
// `updateNewCardRatio` accepts any reduced fraction with
// 1 <= numerator < denominator <= 100, so an off-grid ratio (33/100) is a
// legitimate stored state. That stored value IS a rational number, and every
// question the UI asks about it — is it on the 5% grid, where does the
// control sit, what do the labels read — has an exact integer answer, so the
// helpers below scale to integers before dividing instead of dividing first and
// rounding the IEEE-754 error away afterwards.

/** Lowest new-card share the 5%-step control can select, in percent. */
export const MIN = 5;
/** Highest new-card share the 5%-step control can select, in percent. */
export const MAX = 95;
/** Granularity of the 5%-step control, in percentage points. */
export const STEP = 5;

/** A stored new-card share, kept as the exact fraction `numerator/denominator`. */
export type Ratio = { numerator: number; denominator: number };

/**
 * Exact share of new cards in tenths of a percent (0..1000), round-half-up:
 * 1000n/d rounded is `floor((2000n + d) / 2d)`. Every term is a safe integer
 * because 2000n <= 200000. JS has no integer division, so the quotient is a
 * float — but the `Math.floor` is still exact: a non-integral quotient of these
 * operands sits at least 1/2d away from any integer, and d <= 100 puts that at
 * 0.005 or more, many orders of magnitude beyond double rounding error. An
 * integral quotient divides exactly.
 */
export function shareTenths({ numerator, denominator }: Ratio): number {
  return Math.floor((2000 * numerator + denominator) / (2 * denominator));
}

/**
 * True when the exact share 100n/d is a whole multiple of 5, which happens
 * exactly when d divides 20n. n < d keeps the share strictly inside (0, 100),
 * so an on-grid share always lands within [MIN, MAX] and the clamp never
 * interacts with this test.
 */
export function isOnGrid({ numerator, denominator }: Ratio): boolean {
  return (20 * numerator) % denominator === 0;
}

/**
 * Position of the 5%-step control: the nearest grid step to the exact share,
 * clamped to [MIN, MAX]. The grid index is 20n/d rounded half-up, i.e.
 * `floor((40n + d) / 2d)`. Its `Math.floor` is exact for the same reason as
 * `shareTenths`: these operands put every non-integral quotient at least
 * 1/2d >= 0.005 clear of the nearest integer. The control only positions itself
 * here — it never stands in for the stored value.
 */
export function gridPercent({ numerator, denominator }: Ratio): number {
  const gridIndex = Math.floor((40 * numerator + denominator) / (2 * denominator));
  return Math.min(MAX, Math.max(MIN, STEP * gridIndex));
}

/** True when the stored ratio is exactly `percent`%, i.e. 100n/d === percent. */
export function equalsPercent({ numerator, denominator }: Ratio, percent: number): boolean {
  return 100 * numerator === percent * denominator;
}

/**
 * The number the ICU message interpolates: 800 -> 80, 333 -> 33.3. Dividing a
 * tenths integer by 10 yields the only float that reaches the UI, and it is
 * exact in the only sense that matters here — the quotient is the double
 * nearest that decimal, so it prints back as the same one-decimal string.
 */
export function tenthsToPercent(tenths: number): number {
  return tenths / 10;
}
