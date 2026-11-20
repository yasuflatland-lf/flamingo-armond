import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { mergePrefetchedCards, type SwipedSession } from "./learn-queue";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

type Card = { id: string; front: string };

const TODAY = "2026-07-20";
const YESTERDAY = "2026-07-19";

function makeCards(ids: readonly string[]): Card[] {
  return ids.map((id) => ({ id, front: `Front ${id}` }));
}

function swipes(dayKey: string, ids: readonly string[]): SwipedSession {
  return { dayKey, ids: new Set(ids) };
}

// ---------------------------------------------------------------------------
// Laws (property)
// ---------------------------------------------------------------------------

const ID_POOL = ["a", "b", "c", "d", "e", "f", "g"];
/** Cards with unique ids drawn from ID_POOL (the server never repeats an id in one batch). */
const cardsArb = fc.uniqueArray(fc.constantFrom(...ID_POOL), { maxLength: 5 }).map(makeCards);
const mergeInput = fc.record({
  current: cardsArb,
  incoming: cardsArb,
  swipedIds: fc.uniqueArray(fc.constantFrom(...ID_POOL), { maxLength: 4 }),
  sameDay: fc.boolean(),
});

describe("mergePrefetchedCards laws (property)", () => {
  it("appends, by reference and in server order, exactly the incoming cards neither queued nor swiped this learn day", () => {
    fc.assert(
      fc.property(mergeInput, ({ current, incoming, swipedIds, sameDay }) => {
        const swiped = swipes(sameDay ? TODAY : YESTERDAY, swipedIds);
        const queued = new Set(current.map((c) => c.id));
        const expected = incoming.filter(
          (c) => !queued.has(c.id) && !(sameDay && swiped.ids.has(c.id)),
        );
        const result = mergePrefetchedCards(current, incoming, swiped, TODAY);
        const want = [...current, ...expected];
        expect(result.queue).toHaveLength(want.length);
        for (const [i, card] of want.entries()) expect(result.queue[i]).toBe(card);
        expect(result.exhausted).toBe(expected.length === 0);
      }),
    );
  });

  it("never mutates its inputs and always returns a new queue array", () => {
    fc.assert(
      fc.property(mergeInput, ({ current, incoming, swipedIds, sameDay }) => {
        const swiped = swipes(sameDay ? TODAY : YESTERDAY, swipedIds);
        const snapshot = JSON.stringify([current, incoming, [...swiped.ids]]);
        const result = mergePrefetchedCards(current, incoming, swiped, TODAY);
        expect(JSON.stringify([current, incoming, [...swiped.ids]])).toBe(snapshot);
        expect(result.queue).not.toBe(current);
      }),
    );
  });
});

// ---------------------------------------------------------------------------
// Learn-day rollover
// ---------------------------------------------------------------------------

describe("mergePrefetchedCards across the JST learn-day rollover", () => {
  it("ignores swiped ids recorded on a previous learn day so re-served cards are admitted", () => {
    const current = makeCards(["a"]);
    const incoming = makeCards(["x", "y"]);
    const result = mergePrefetchedCards(current, incoming, swipes(YESTERDAY, ["x", "y"]), TODAY);

    expect(result.exhausted).toBe(false);
    expect(result.queue.map((c) => c.id)).toEqual(["a", "x", "y"]);
  });

  it("still honours the queued-card filter when the swiped ids are stale", () => {
    const current = makeCards(["x"]);
    const incoming = makeCards(["x", "y"]);
    const result = mergePrefetchedCards(current, incoming, swipes(YESTERDAY, ["x", "y"]), TODAY);

    expect(result.exhausted).toBe(false);
    expect(result.queue.map((c) => c.id)).toEqual(["x", "y"]);
  });

  it("honours the same ids while the recorded day still matches", () => {
    const current = makeCards(["a"]);
    const incoming = makeCards(["x", "y"]);
    const result = mergePrefetchedCards(current, incoming, swipes(TODAY, ["x", "y"]), TODAY);

    expect(result.exhausted).toBe(true);
    expect(result.queue.map((c) => c.id)).toEqual(["a"]);
  });
});
