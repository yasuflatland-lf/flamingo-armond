import { InMemoryCache } from "@apollo/client";
import { describe, expect, it, vi } from "vitest";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import {
  appendConnectionEdge,
  removeConnectionEdgeAcrossVariants,
  removeConnectionEdges,
} from "./connection-cache";

const CARDGROUP_ID = "cg-1";

const VARS: CardsByCardgroupConnectionQueryVariables = {
  cardgroupId: CARDGROUP_ID,
  first: 20,
  search: null,
};

type CardNode =
  CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"]["edges"][number]["node"];

function makeNode(id: string): CardNode {
  return {
    __typename: "Card",
    id,
    front: `front-${id}`,
    back: `back-${id}`,
    cardgroupId: CARDGROUP_ID,
    userCardState: { __typename: "UserCardState", due: "2026-01-01T00:00:00Z", state: 0 },
  };
}

function makeEdge(id: string) {
  return { __typename: "CardEdge" as const, cursor: id, node: makeNode(id) };
}

function makeConnection(
  ids: string[],
  totalCount = ids.length,
): CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"] {
  return {
    __typename: "CardConnection",
    edges: ids.map(makeEdge),
    pageInfo: {
      __typename: "PageInfo",
      hasNextPage: false,
      hasPreviousPage: false,
      startCursor: ids[0] ?? null,
      endCursor: ids.at(-1) ?? null,
    },
    totalCount,
  };
}

function seed(
  cache: InMemoryCache,
  connection: CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"],
) {
  cache.writeQuery({
    query: CardsByCardgroupConnectionDocument,
    variables: VARS,
    data: { cardsByCardgroupConnection: connection },
  });
}

function read(cache: InMemoryCache) {
  return cache.readQuery({ query: CardsByCardgroupConnectionDocument, variables: VARS });
}

const buildColdConnection = () => makeConnection(["cold"], 1);

describe("appendConnectionEdge", () => {
  it("no-ops on a cold cache when buildColdConnection is omitted", () => {
    const cache = new InMemoryCache();

    appendConnectionEdge(cache, {
      document: CardsByCardgroupConnectionDocument,
      variables: VARS,
      connectionField: "cardsByCardgroupConnection",
      edgeTypename: "CardEdge",
      node: makeNode("new"),
    });

    expect(read(cache)).toBeNull();
  });

  it("seeds a fresh connection on a cold cache when buildColdConnection is provided", () => {
    const cache = new InMemoryCache();

    appendConnectionEdge(cache, {
      document: CardsByCardgroupConnectionDocument,
      variables: VARS,
      connectionField: "cardsByCardgroupConnection",
      edgeTypename: "CardEdge",
      node: makeNode("cold"),
      buildColdConnection,
    });

    const result = read(cache);
    expect(result?.cardsByCardgroupConnection.edges.map((e) => e.node.id)).toEqual(["cold"]);
    expect(result?.cardsByCardgroupConnection.totalCount).toBe(1);
  });

  it("prepends a new edge and bumps totalCount on a warm cache", () => {
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a", "b"], 2));

    appendConnectionEdge(cache, {
      document: CardsByCardgroupConnectionDocument,
      variables: VARS,
      connectionField: "cardsByCardgroupConnection",
      edgeTypename: "CardEdge",
      node: makeNode("new"),
    });

    const result = read(cache);
    expect(result?.cardsByCardgroupConnection.edges.map((e) => e.node.id)).toEqual([
      "new",
      "a",
      "b",
    ]);
    expect(result?.cardsByCardgroupConnection.totalCount).toBe(3);
  });

  it("skips the write when an edge with the same node.id already exists (dedup guard)", () => {
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a", "b"], 2));

    appendConnectionEdge(cache, {
      document: CardsByCardgroupConnectionDocument,
      variables: VARS,
      connectionField: "cardsByCardgroupConnection",
      edgeTypename: "CardEdge",
      node: makeNode("a"),
    });

    const result = read(cache);
    expect(result?.cardsByCardgroupConnection.edges.map((e) => e.node.id)).toEqual(["a", "b"]);
    expect(result?.cardsByCardgroupConnection.totalCount).toBe(2);
  });
});

describe("removeConnectionEdges", () => {
  it("removes a single edge and decrements totalCount (per-row delete)", () => {
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a", "b", "c"], 3));

    removeConnectionEdges(cache, {
      document: CardsByCardgroupConnectionDocument,
      variables: VARS,
      connectionField: "cardsByCardgroupConnection",
      entityTypename: "Card",
      ids: ["b"],
      deletedCount: 1,
    });

    const result = read(cache);
    expect(result?.cardsByCardgroupConnection.edges.map((e) => e.node.id)).toEqual(["a", "c"]);
    expect(result?.cardsByCardgroupConnection.totalCount).toBe(2);
  });

  it("removes many edges in one call (bulk delete)", () => {
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a", "b", "c"], 3));

    removeConnectionEdges(cache, {
      document: CardsByCardgroupConnectionDocument,
      variables: VARS,
      connectionField: "cardsByCardgroupConnection",
      entityTypename: "Card",
      ids: ["a", "c"],
      deletedCount: 2,
    });

    const result = read(cache);
    expect(result?.cardsByCardgroupConnection.edges.map((e) => e.node.id)).toEqual(["b"]);
    expect(result?.cardsByCardgroupConnection.totalCount).toBe(1);
  });

  it("clamps totalCount at 0 when deletedCount exceeds the loaded edge count", () => {
    // Only one edge is in the cache, but the server deleted 5 (the rest live on
    // pages that were never fetched into `edges`). The clamp keeps totalCount >= 0.
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a"], 3));

    removeConnectionEdges(cache, {
      document: CardsByCardgroupConnectionDocument,
      variables: VARS,
      connectionField: "cardsByCardgroupConnection",
      entityTypename: "Card",
      ids: ["a"],
      deletedCount: 5,
    });

    const result = read(cache);
    expect(result?.cardsByCardgroupConnection.edges).toEqual([]);
    expect(result?.cardsByCardgroupConnection.totalCount).toBe(0);
  });

  it("evicts the normalized entities and runs gc", () => {
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a", "b"], 2));
    const evictSpy = vi.spyOn(cache, "evict");
    const gcSpy = vi.spyOn(cache, "gc");

    removeConnectionEdges(cache, {
      document: CardsByCardgroupConnectionDocument,
      variables: VARS,
      connectionField: "cardsByCardgroupConnection",
      entityTypename: "Card",
      ids: ["a"],
      deletedCount: 1,
    });

    expect(evictSpy).toHaveBeenCalledWith({ id: "Card:a" });
    expect(gcSpy).toHaveBeenCalledTimes(1);
    // The normalized Card:a entry is gone after evict + gc.
    expect(cache.extract()["Card:a"]).toBeUndefined();
  });

  it("still evicts + gcs on a cold cache (no connection seeded)", () => {
    const cache = new InMemoryCache();
    const evictSpy = vi.spyOn(cache, "evict");
    const gcSpy = vi.spyOn(cache, "gc");

    removeConnectionEdges(cache, {
      document: CardsByCardgroupConnectionDocument,
      variables: VARS,
      connectionField: "cardsByCardgroupConnection",
      entityTypename: "Card",
      ids: ["a"],
      deletedCount: 1,
    });

    expect(evictSpy).toHaveBeenCalledWith({ id: "Card:a" });
    expect(gcSpy).toHaveBeenCalledTimes(1);
  });
});

describe("removeConnectionEdgeAcrossVariants", () => {
  // A second cached variant of the same field, keyed on a different `search`
  // argument. `cache.modify` visits every cached instance of the field, so a
  // single call must drop the entity from both variants at once.
  const FILTERED_VARS: CardsByCardgroupConnectionQueryVariables = {
    cardgroupId: CARDGROUP_ID,
    first: 20,
    search: "a",
  };

  function seedVariant(
    cache: InMemoryCache,
    variables: CardsByCardgroupConnectionQueryVariables,
    ids: string[],
    totalCount = ids.length,
  ) {
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables,
      data: { cardsByCardgroupConnection: makeConnection(ids, totalCount) },
    });
  }

  function readVariant(cache: InMemoryCache, variables: CardsByCardgroupConnectionQueryVariables) {
    return cache.readQuery({ query: CardsByCardgroupConnectionDocument, variables });
  }

  it("removes the matching edge from every cached variant and clamps totalCount at 0", () => {
    const cache = new InMemoryCache();
    seedVariant(cache, VARS, ["a", "b", "c"], 3);
    // The filtered variant under-counts totalCount (1) relative to its loaded
    // edges, so removing "b" exercises the Math.max(0, …) clamp.
    seedVariant(cache, FILTERED_VARS, ["a", "b"], 1);

    removeConnectionEdgeAcrossVariants(cache, {
      connectionField: "cardsByCardgroupConnection",
      entityTypename: "Card",
      id: "b",
    });

    const nullVariant = readVariant(cache, VARS);
    expect(nullVariant?.cardsByCardgroupConnection.edges.map((e) => e.node.id)).toEqual(["a", "c"]);
    expect(nullVariant?.cardsByCardgroupConnection.totalCount).toBe(2);

    const filteredVariant = readVariant(cache, FILTERED_VARS);
    expect(filteredVariant?.cardsByCardgroupConnection.edges.map((e) => e.node.id)).toEqual(["a"]);
    expect(filteredVariant?.cardsByCardgroupConnection.totalCount).toBe(0);
  });

  it("evicts the normalized entity and runs gc", () => {
    const cache = new InMemoryCache();
    seedVariant(cache, VARS, ["a", "b"], 2);
    const evictSpy = vi.spyOn(cache, "evict");
    const gcSpy = vi.spyOn(cache, "gc");

    removeConnectionEdgeAcrossVariants(cache, {
      connectionField: "cardsByCardgroupConnection",
      entityTypename: "Card",
      id: "a",
    });

    expect(evictSpy).toHaveBeenCalledWith({ id: "Card:a" });
    expect(gcSpy).toHaveBeenCalledTimes(1);
    expect(cache.extract()["Card:a"]).toBeUndefined();
  });

  it("leaves a variant that does not contain the id untouched", () => {
    const cache = new InMemoryCache();
    seedVariant(cache, VARS, ["x", "y"], 2);

    removeConnectionEdgeAcrossVariants(cache, {
      connectionField: "cardsByCardgroupConnection",
      entityTypename: "Card",
      id: "z",
    });

    const result = readVariant(cache, VARS);
    expect(result?.cardsByCardgroupConnection.edges.map((e) => e.node.id)).toEqual(["x", "y"]);
    expect(result?.cardsByCardgroupConnection.totalCount).toBe(2);
  });
});
