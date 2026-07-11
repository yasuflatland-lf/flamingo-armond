/**
 * Direction the learner rated a card with, derived from a horizontal or
 * downward swipe gesture (or the equivalent action-bar button / arrow key):
 * - "left"  -> Again
 * - "down"  -> Hard
 * - "right" -> Easy
 *
 * Owned by the learn component layer so low-level UI primitives
 * (SwipeCardStack, AnimatedCard, SwipeDirectionOverlay, LearnActionBar)
 * do not depend on the higher-level route client.
 */
export type SwipeDirection = "left" | "right" | "down";

/**
 * Color/label identity shared by a rating across surfaces. The string also
 * doubles as the next-intl message key under the `Learn` namespace
 * (`again`/`hard`/`easy`), so a single token drives both the localized label
 * and the per-surface color class map.
 */
export type RatingTone = "again" | "hard" | "easy";

/**
 * Single source of truth for the swipe-rating semantics. Both `LearnActionBar`
 * and `SwipeDirectionOverlay` read the label key and color tone from here so a
 * rating's wording or color identity is defined once. Each surface keeps its
 * own literal Tailwind class strings keyed by `tone` (a 56px circular button's
 * classes differ from the overlay chip's) — only the label + color *identity*
 * is shared, not the literal classes.
 */
export const RATING_META: Record<SwipeDirection, { labelKey: RatingTone; tone: RatingTone }> = {
  left: { labelKey: "again", tone: "again" },
  down: { labelKey: "hard", tone: "hard" },
  right: { labelKey: "easy", tone: "easy" },
};

/**
 * FSRS swipe rating emitted for each rating tone. The 3-step UI emits
 * 1=Again, 2=Hard, 4=Easy and intentionally never 3=Good. Keyed by `RatingTone`
 * so the direction→rating mapping derives from `RATING_META`
 * (`SWIPE_RATING[RATING_META[direction].tone]`) rather than a second, parallel
 * direction switch with bare integer literals.
 */
export const SWIPE_RATING: Record<RatingTone, 1 | 2 | 4> = {
  again: 1,
  hard: 2,
  easy: 4,
};

/**
 * Re-export of the generated GraphQL `LearnDisplayMode` enum so the schema is
 * the single source of truth and the component layer can never drift from it.
 */
export type { LearnDisplayMode } from "@/generated/graphql";

/**
 * Reveal phase of the active card.
 * - "front_only": flip mode, back hidden, rating swipes suppressed, tap reveals.
 * - "revealed": back shown, rating swipes active. always_visible starts here.
 */
export type LearnCardPhase = "front_only" | "revealed";
