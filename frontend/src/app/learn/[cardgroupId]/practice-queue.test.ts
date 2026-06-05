import { describe, expect, it } from "vitest";
import {
  PRACTICE_REQUEUE_OFFSET,
  advancePracticeQueue,
  outcomeFromDirection,
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
// advancePracticeQueue — again
// ---------------------------------------------------------------------------

describe('advancePracticeQueue with outcome "again"', () => {
  it("removes the card from index 0 and re-inserts it at PRACTICE_REQUEUE_OFFSET", () => {
    // Long enough queue: 10 cards, target is head (index 0)
    const queue = makeCards(10);
    const result = advancePracticeQueue(queue, "card-1", "again");

    // card-1 should no longer be at index 0
    expect(result[0].id).not.toBe("card-1");
    // card-1 should appear at index PRACTICE_REQUEUE_OFFSET (5)
    expect(result[PRACTICE_REQUEUE_OFFSET].id).toBe("card-1");
    // Length unchanged
    expect(result).toHaveLength(queue.length);
  });

  it("re-inserts a mid-queue card at the correct offset", () => {
    // card-3 is at index 2 in a 10-card queue; after removal length = 9;
    // re-insert at min(5, 9) = 5
    const queue = makeCards(10);
    const cardId = "card-3";
    const result = advancePracticeQueue(queue, cardId, "again");

    expect(result).toHaveLength(queue.length);
    expect(result[PRACTICE_REQUEUE_OFFSET].id).toBe(cardId);
    // The removed card must not appear elsewhere
    const positions = result.reduce<number[]>((acc, c, i) => {
      if (c.id === cardId) acc.push(i);
      return acc;
    }, []);
    expect(positions).toEqual([PRACTICE_REQUEUE_OFFSET]);
  });
});

// ---------------------------------------------------------------------------
// advancePracticeQueue — hard
// ---------------------------------------------------------------------------

describe('advancePracticeQueue with outcome "hard"', () => {
  it("treats hard identically to again: re-inserts at PRACTICE_REQUEUE_OFFSET", () => {
    const queue = makeCards(10);
    const result = advancePracticeQueue(queue, "card-1", "hard");

    expect(result[0].id).not.toBe("card-1");
    expect(result[PRACTICE_REQUEUE_OFFSET].id).toBe("card-1");
    expect(result).toHaveLength(queue.length);
  });
});

// ---------------------------------------------------------------------------
// advancePracticeQueue — easy
// ---------------------------------------------------------------------------

describe('advancePracticeQueue with outcome "easy"', () => {
  it("removes the card entirely from the queue", () => {
    const queue = makeCards(5);
    const result = advancePracticeQueue(queue, "card-1", "easy");

    expect(result).toHaveLength(queue.length - 1);
    expect(result.some((c) => c.id === "card-1")).toBe(false);
  });

  it("retires a mid-queue card without affecting the relative order of the others", () => {
    const queue = makeCards(6);
    const result = advancePracticeQueue(queue, "card-3", "easy");

    expect(result).toHaveLength(queue.length - 1);
    const ids = result.map((c) => c.id);
    expect(ids).toEqual(["card-1", "card-2", "card-4", "card-5", "card-6"]);
  });
});

// ---------------------------------------------------------------------------
// Tail clamping
// ---------------------------------------------------------------------------

describe("tail clamping when queue is shorter than offset after removal", () => {
  it("lands the card at the end when queue length after removal < PRACTICE_REQUEUE_OFFSET", () => {
    // 3-card queue → after removal length = 2; offset (5) > 2, clamp to 2 (end)
    const queue = makeCards(3);
    const result = advancePracticeQueue(queue, "card-1", "again");

    expect(result).toHaveLength(3);
    // card-1 should be at the last position
    expect(result[result.length - 1].id).toBe("card-1");
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
    expect(result[0].id).toBe("card-1");
  });
});

// ---------------------------------------------------------------------------
// cardId not found (defensive no-op)
// ---------------------------------------------------------------------------

describe("cardId not found in queue", () => {
  it("returns equal contents when cardId is not present", () => {
    const queue = makeCards(4);
    const result = advancePracticeQueue(queue, "nonexistent-id", "again");

    expect(result.map((c) => c.id)).toEqual(queue.map((c) => c.id));
  });

  it("returns a different array instance (shallow copy) even when cardId is absent", () => {
    const queue = makeCards(4);
    const result = advancePracticeQueue(queue, "nonexistent-id", "again");

    expect(result).not.toBe(queue);
  });
});

// ---------------------------------------------------------------------------
// Immutability
// ---------------------------------------------------------------------------

describe("immutability", () => {
  const outcomes: PracticeOutcome[] = ["again", "hard", "easy"];

  for (const outcome of outcomes) {
    it(`does not mutate the input array for outcome "${outcome}"`, () => {
      const queue = makeCards(8);
      const originalIds = queue.map((c) => c.id);
      advancePracticeQueue(queue, "card-1", outcome);

      // Same order and contents
      expect(queue.map((c) => c.id)).toEqual(originalIds);
    });

    it(`returns a new array instance for outcome "${outcome}"`, () => {
      const queue = makeCards(8);
      const result = advancePracticeQueue(queue, "card-1", outcome);
      expect(result).not.toBe(queue);
    });
  }
});
