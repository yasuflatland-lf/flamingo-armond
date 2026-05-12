/**
 * Direction the learner rated a card with, derived from a horizontal or
 * downward swipe gesture (or the equivalent action-bar button / arrow key):
 * - "left"  -> Again
 * - "down"  -> Hard
 * - "right" -> Easy
 *
 * Owned by the learn component layer so low-level UI primitives
 * (SwipeCardStack, AnimatedCard, SwipeProgressOverlay, LearnActionBar)
 * do not depend on the higher-level route client.
 */
export type SwipeDirection = "left" | "right" | "down";
