import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { makeMergeConnection } from "./make-merge-connection";

interface Edge {
  cursor: string;
  node: { id: string };
}

interface PageInfo {
  hasNextPage: boolean;
  endCursor: string | null;
}

interface Result {
  items: {
    __typename: "ItemConnection";
    edges: Edge[];
    pageInfo: PageInfo;
    totalCount: number;
  };
}

const edge = (id: string): Edge => ({ cursor: id, node: { id } });

function page(edges: Edge[], pageInfo: PageInfo, totalCount: number): Result {
  return {
    items: { __typename: "ItemConnection", edges, pageInfo, totalCount },
  };
}

describe("makeMergeConnection", () => {
  const merge = makeMergeConnection<Result>("items");

  it("concatenates prev ++ more edges, takes pageInfo and totalCount from more, and leaves both inputs untouched (property)", () => {
    const pageArb = fc.record({
      ids: fc.array(fc.uuid(), { maxLength: 5 }),
      hasNextPage: fc.boolean(),
      totalCount: fc.nat(50),
    });
    fc.assert(
      fc.property(pageArb, pageArb, (a, b) => {
        const prev = page(
          a.ids.map(edge),
          { hasNextPage: a.hasNextPage, endCursor: a.ids.at(-1) ?? null },
          a.totalCount,
        );
        const more = page(
          b.ids.map(edge),
          { hasNextPage: b.hasNextPage, endCursor: b.ids.at(-1) ?? null },
          b.totalCount,
        );
        const snapshot = JSON.stringify([prev, more]);
        const merged = merge(prev, more);
        expect(merged.items.edges).toEqual([...prev.items.edges, ...more.items.edges]);
        expect(merged.items.pageInfo).toBe(more.items.pageInfo);
        expect(merged.items.totalCount).toBe(more.items.totalCount);
        expect(merged.items.__typename).toBe("ItemConnection");
        expect(JSON.stringify([prev, more])).toBe(snapshot);
      }),
    );
  });

  it("carries sibling top-level fields of the incoming result through", () => {
    interface WithSibling extends Result {
      unrelated: string;
    }
    const mergeWithSibling = makeMergeConnection<WithSibling>("items");
    const prev: WithSibling = {
      ...page([edge("a")], { hasNextPage: true, endCursor: "a" }, 2),
      unrelated: "stale",
    };
    const more: WithSibling = {
      ...page([edge("b")], { hasNextPage: false, endCursor: "b" }, 2),
      unrelated: "fresh",
    };

    expect(mergeWithSibling(prev, more).unrelated).toBe("fresh");
  });

  it("returns a fresh reducer per call, so consumers must hold one instance", () => {
    expect(makeMergeConnection<Result>("items")).not.toBe(makeMergeConnection<Result>("items"));
  });
});
