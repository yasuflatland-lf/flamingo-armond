import fc from "fast-check";
import { describe, expect, it } from "vitest";
import {
  advancePracticeQueue,
  outcomeFromDirection,
  PRACTICE_REQUEUE_OFFSET,
  type PracticeOutcome,
} from "./practice-queue";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

type Card = { id: string; front: string };

function makeCards(count: number): Card[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `card-${i + 1}`,
    front: `Front ${i + 1}`,
  }));
}

// ---------------------------------------------------------------------------
// outcomeFromDirection
// ---------------------------------------------------------------------------

describe("outcomeFromDirection", () => {
  it('maps "left" to "again"', () => {
    expect(outcomeFromDirection("left")).toBe("again");
  });

  it('maps "down" to "hard"', () => {
    expect(outcomeFromDirection("down")).toBe("hard");
  });

  it('maps "right" to "easy"', () => {
    expect(outcomeFromDirection("right")).toBe("easy");
  });
});

// ---------------------------------------------------------------------------
// advancePracticeQueue — laws (property)
// ---------------------------------------------------------------------------

/** A queue of 1..12 cards with unique ids, plus the id of one of them. */
const queueWithTarget = fc
  .integer({ min: 1, max: 12 })
  .chain((n) => fc.record({ queue: fc.constant(makeCards(n)), i: fc.integer({ min: 1, max: n }) }))
  .map(({ queue, i }) => ({ queue, cardId: `card-${i}` }));
const ids = (cards: readonly Card[]) => cards.map((c) => c.id);
const others = (cards: readonly Card[], id: string) => ids(cards).filter((x) => x !== id);

describe("advancePracticeQueue laws (property)", () => {
  it("again/hard moves the card to min(PRACTICE_REQUEUE_OFFSET, length - 1) and keeps the others' order", () => {
    const requeue = fc.constantFrom<PracticeOutcome>("again", "hard");
    fc.assert(
      fc.property(queueWithTarget, requeue, ({ queue, cardId }, outcome) => {
        const result = advancePracticeQueue(queue, cardId, outcome);
        expect(result).toHaveLength(queue.length);
        expect(ids(result).indexOf(cardId)).toBe(
          Math.min(PRACTICE_REQUEUE_OFFSET, queue.length - 1),
        );
        expect(others(result, cardId)).toEqual(others(queue, cardId));
      }),
    );
  });

  it("easy removes exactly the rated card and keeps the others' order", () => {
    fc.assert(
      fc.property(queueWithTarget, ({ queue, cardId }) => {
        expect(ids(advancePracticeQueue(queue, cardId, "easy"))).toEqual(others(queue, cardId));
      }),
    );
  });

  it("never mutates its input and returns a new array; an absent id yields an equal copy", () => {
    const outcome = fc.constantFrom<PracticeOutcome>("again", "hard", "easy");
    fc.assert(
      fc.property(queueWithTarget, outcome, fc.boolean(), ({ queue, cardId }, o, absent) => {
        const before = ids(queue);
        const result = advancePracticeQueue(queue, absent ? "nonexistent-id" : cardId, o);
        expect(ids(queue)).toEqual(before);
        expect(result).not.toBe(queue);
        if (absent) expect(ids(result)).toEqual(before);
      }),
    );
  });
});

// ---------------------------------------------------------------------------
// Single-card queue
// ---------------------------------------------------------------------------

describe("single-card queue with again", () => {
  it("keeps the card as the only element (offset clamps to 0)", () => {
    const queue: Card[] = [{ id: "card-1", front: "Front 1" }];
    const result = advancePracticeQueue(queue, "card-1", "again");

    expect(result).toHaveLength(1);
    const only = result[0];
    expect(only?.id).toBe("card-1");
  });
});
