// Exact-rational arithmetic for the share of a learn session using new cards.
// `updateNewCardRatio` accepts reduced fractions whose denominator divides the
// 20-card default session, so the on-grid 13/20 (65%) is a legitimate stored
// state. Every UI question about its grid position or label has an exact integer
// answer, so these helpers scale before dividing to avoid rounding error.

/** Lowest new-card share the 5%-step control can select, in percent. */
export const MIN = 5;
/**
 * Highest new-card share the 5%-step control can select, in percent. Capped at
 * 80% so the review share stays >= 20% — the FSRS discovery-first floor; the
 * backend `NewCardRatio` value object enforces the same bound authoritatively.
 */
export const MAX = 80;
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
 * Nearest 5%-step position, clamped to [MIN, MAX]. An exact share is on the grid
 * iff d divides 20n; alignment does not imply it is selectable. The grid index
 * is `floor((40n + d) / 2d)`, exact for the same reason as `shareTenths`.
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
