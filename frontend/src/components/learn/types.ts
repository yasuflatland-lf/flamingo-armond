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
