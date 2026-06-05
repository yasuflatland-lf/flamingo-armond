import type { SwipeDirection } from "@/components/learn/types";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type PracticeOutcome = "again" | "hard" | "easy";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/**
 * How many positions ahead to re-insert a card when the learner answers
 * Again or Hard.  Five is a deliberate choice: far enough that the learner
 * will encounter a few new cards before seeing the same card again, close
 * enough that the repetition still happens within the same short session.
 */
export const PRACTICE_REQUEUE_OFFSET = 5;

// ---------------------------------------------------------------------------
// outcomeFromDirection
// ---------------------------------------------------------------------------

/**
 * Converts a swipe direction into a practice outcome.
 *
 * Mapping:
 *   "left"  → "again"  (swiped left = didn't know it)
 *   "down"  → "hard"   (swiped down = knew it but struggled)
 *   "right" → "easy"   (swiped right = knew it confidently)
 */
export function outcomeFromDirection(direction: SwipeDirection): PracticeOutcome {
  switch (direction) {
    case "left":
      return "again";
    case "down":
      return "hard";
    case "right":
      return "easy";
  }
}

// ---------------------------------------------------------------------------
// advancePracticeQueue
// ---------------------------------------------------------------------------

/**
 * Pure, side-effect-free transition of the practice queue after the learner
 * rates a card.
 *
 * Deliberate two-state reduction
 * ───────────────────────────────
 * Again and Hard both map to "one more time this session".  This mirrors the
 * backend's FSRS learning-phase classification where FSRSStateLearning and
 * FSRSStateRelearning both indicate a card is not yet mastered.  The policy
 * does not need to distinguish between them at the queue level: regardless of
 * which rating the learner gave, the card re-enters the queue a few positions
 * later so the learner sees it again before the session ends.
 *
 * Easy maps to "done for this round": the card is retired from the queue and
 * will not appear again until the next session.
 *
 * @param queue   Current ordered queue.  Never mutated.  Card IDs must be
 *                unique within the queue: only the first match is moved, and
 *                duplicate IDs are not supported.
 * @param cardId  ID of the card that was just rated.
 * @param outcome Result of the rating (again | hard | easy).
 * @returns       A new array representing the updated queue.
 */
export function advancePracticeQueue<T extends { id: string }>(
  queue: readonly T[],
  cardId: string,
  outcome: PracticeOutcome,
): T[] {
  const idx = queue.findIndex((c) => c.id === cardId);

  // Defensive no-op: cardId not found → return a shallow copy unchanged.
  if (idx === -1) {
    return [...queue];
  }

  // Hoist to a checked local so the type is T, not T | undefined.
  // idx is guaranteed to be a valid index because findIndex returned >= 0.
  const card = queue[idx];
  if (card === undefined) {
    // Structurally unreachable: findIndex only returns a non-negative index
    // when an element exists at that position.  Guard is here solely to
    // satisfy noUncheckedIndexedAccess without an unsafe cast.
    return [...queue];
  }

  // Remove the card from its current position.
  const without = [...queue.slice(0, idx), ...queue.slice(idx + 1)];

  if (outcome === "easy") {
    // Retire the card — it leaves the queue entirely.
    return without;
  }

  // again / hard: re-insert at min(PRACTICE_REQUEUE_OFFSET, without.length)
  // so the card always lands within bounds even when the queue is short.
  const insertAt = Math.min(PRACTICE_REQUEUE_OFFSET, without.length);
  return [...without.slice(0, insertAt), card, ...without.slice(insertAt)];
}
