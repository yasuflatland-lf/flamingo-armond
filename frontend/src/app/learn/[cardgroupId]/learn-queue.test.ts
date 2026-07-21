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

const NOTHING_SWIPED: SwipedSession = swipes(TODAY, []);

// ---------------------------------------------------------------------------
// Verdict 1 — empty incoming
// ---------------------------------------------------------------------------

describe("mergePrefetchedCards with an empty incoming batch", () => {
  it("marks the pool exhausted and leaves the queue contents unchanged", () => {
    const current = makeCards(["a", "b"]);
    const result = mergePrefetchedCards(current, [], NOTHING_SWIPED, TODAY);

    expect(result.exhausted).toBe(true);
    expect(result.queue.map((c) => c.id)).toEqual(["a", "b"]);
  });

  it("returns the same verdict for an empty queue and an empty batch", () => {
    const result = mergePrefetchedCards<Card>([], [], NOTHING_SWIPED, TODAY);

    expect(result.exhausted).toBe(true);
    expect(result.queue).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// Verdict 2 — every incoming row filtered out
// ---------------------------------------------------------------------------

describe("mergePrefetchedCards when every incoming row is filtered out", () => {
  it("marks the pool exhausted when the batch only repeats queued cards", () => {
    const current = makeCards(["a", "b"]);
    const incoming = makeCards(["a", "b"]);
    const result = mergePrefetchedCards(current, incoming, NOTHING_SWIPED, TODAY);

    expect(result.exhausted).toBe(true);
    expect(result.queue.map((c) => c.id)).toEqual(["a", "b"]);
  });

  it("marks the pool exhausted when the batch only repeats cards swiped this learn day", () => {
    const current = makeCards(["a"]);
    const incoming = makeCards(["x", "y"]);
    const result = mergePrefetchedCards(current, incoming, swipes(TODAY, ["x", "y"]), TODAY);

    expect(result.exhausted).toBe(true);
    expect(result.queue.map((c) => c.id)).toEqual(["a"]);
  });

  it("marks the pool exhausted when the queued and swiped filters together cover the batch", () => {
    const current = makeCards(["a"]);
    const incoming = makeCards(["a", "x"]);
    const result = mergePrefetchedCards(current, incoming, swipes(TODAY, ["x"]), TODAY);

    expect(result.exhausted).toBe(true);
    expect(result.queue.map((c) => c.id)).toEqual(["a"]);
  });
});

// ---------------------------------------------------------------------------
// Verdict 3 — partial additions
// ---------------------------------------------------------------------------

describe("mergePrefetchedCards when some incoming rows survive the filter", () => {
  it("appends the survivors to the tail in server order and clears the verdict", () => {
    const current = makeCards(["a", "b"]);
    const incoming = makeCards(["b", "c", "d"]);
    const result = mergePrefetchedCards(current, incoming, NOTHING_SWIPED, TODAY);

    expect(result.exhausted).toBe(false);
    expect(result.queue.map((c) => c.id)).toEqual(["a", "b", "c", "d"]);
  });

  it("keeps the survivors' relative order when a swiped id sits between them", () => {
    const current = makeCards(["a"]);
    const incoming = makeCards(["c", "x", "b"]);
    const result = mergePrefetchedCards(current, incoming, swipes(TODAY, ["x"]), TODAY);

    expect(result.exhausted).toBe(false);
    expect(result.queue.map((c) => c.id)).toEqual(["a", "c", "b"]);
  });

  it("appends every row when the queue is empty and nothing was swiped", () => {
    const incoming = makeCards(["a", "b"]);
    const result = mergePrefetchedCards<Card>([], incoming, NOTHING_SWIPED, TODAY);

    expect(result.exhausted).toBe(false);
    expect(result.queue.map((c) => c.id)).toEqual(["a", "b"]);
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

// ---------------------------------------------------------------------------
// Immutability
// ---------------------------------------------------------------------------

describe("mergePrefetchedCards immutability", () => {
  const cases: ReadonlyArray<{ name: string; incoming: readonly string[] }> = [
    { name: "empty incoming", incoming: [] },
    { name: "fully filtered incoming", incoming: ["a", "b"] },
    { name: "partially added incoming", incoming: ["b", "c"] },
  ];

  for (const { name, incoming } of cases) {
    it(`does not mutate its inputs for ${name}`, () => {
      const current = makeCards(["a", "b"]);
      const batch = makeCards(incoming);
      const currentIds = current.map((c) => c.id);
      const batchIds = batch.map((c) => c.id);
      const swiped = swipes(TODAY, ["z"]);

      mergePrefetchedCards(current, batch, swiped, TODAY);

      expect(current.map((c) => c.id)).toEqual(currentIds);
      expect(batch.map((c) => c.id)).toEqual(batchIds);
      expect([...swiped.ids]).toEqual(["z"]);
    });

    it(`returns a new array instance for ${name}`, () => {
      const current = makeCards(["a", "b"]);
      const batch = makeCards(incoming);
      const result = mergePrefetchedCards(current, batch, NOTHING_SWIPED, TODAY);

      expect(result.queue).not.toBe(current);
    });
  }

  it("reuses the element references it carries over", () => {
    const current = makeCards(["a"]);
    const incoming = makeCards(["b"]);
    const result = mergePrefetchedCards(current, incoming, NOTHING_SWIPED, TODAY);

    expect(result.queue[0]).toBe(current[0]);
    expect(result.queue[1]).toBe(incoming[0]);
  });
});
