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

  it("concatenates the incoming page's edges after the cached ones", () => {
    const prev = page([edge("a"), edge("b")], { hasNextPage: true, endCursor: "b" }, 4);
    const more = page([edge("c"), edge("d")], { hasNextPage: false, endCursor: "d" }, 4);

    expect(merge(prev, more).items.edges.map((e) => e.node.id)).toEqual(["a", "b", "c", "d"]);
  });

  it("keeps the incoming page's pageInfo and totalCount", () => {
    const prev = page([edge("a")], { hasNextPage: true, endCursor: "a" }, 3);
    const more = page([edge("b")], { hasNextPage: false, endCursor: "b" }, 2);

    const merged = merge(prev, more);
    expect(merged.items.pageInfo).toEqual({ hasNextPage: false, endCursor: "b" });
    expect(merged.items.totalCount).toBe(2);
    expect(merged.items.__typename).toBe("ItemConnection");
  });

  it("does not mutate either input", () => {
    const prev = page([edge("a")], { hasNextPage: true, endCursor: "a" }, 2);
    const more = page([edge("b")], { hasNextPage: false, endCursor: "b" }, 2);

    merge(prev, more);
    expect(prev.items.edges).toHaveLength(1);
    expect(more.items.edges).toHaveLength(1);
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
