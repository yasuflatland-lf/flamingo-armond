import { InMemoryCache } from "@apollo/client";
import { describe, expect, it } from "vitest";
import {
  MyCardgroupsConnectionDocument,
  type MyCardgroupsConnectionQuery,
  type MyCardgroupsConnectionQueryVariables,
} from "@/generated/graphql";
import { prependMyCardgroupEdge, removeMyCardgroupEdge, restoreMyCardgroupSnapshot } from "./cache";

const VARS: MyCardgroupsConnectionQueryVariables = {
  first: 20,
  search: null,
};

type CardgroupNode = MyCardgroupsConnectionQuery["myCardgroupsConnection"]["edges"][number]["node"];

function makeNode(id: string): CardgroupNode {
  return {
    __typename: "Cardgroup",
    id,
    name: `name-${id}`,
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

function makeEdge(id: string) {
  return { __typename: "CardgroupEdge" as const, cursor: `v1:${id}`, node: makeNode(id) };
}

function makeConnection(
  ids: string[],
  totalCount = ids.length,
): MyCardgroupsConnectionQuery["myCardgroupsConnection"] {
  return {
    __typename: "CardgroupConnection",
    edges: ids.map(makeEdge),
    pageInfo: {
      __typename: "PageInfo",
      hasNextPage: false,
      hasPreviousPage: false,
      startCursor: ids[0] ? `v1:${ids[0]}` : null,
      endCursor: ids.at(-1) ? `v1:${ids.at(-1)}` : null,
    },
    totalCount,
  };
}

function seed(
  cache: InMemoryCache,
  connection: MyCardgroupsConnectionQuery["myCardgroupsConnection"],
) {
  cache.writeQuery({
    query: MyCardgroupsConnectionDocument,
    variables: VARS,
    data: { myCardgroupsConnection: connection },
  });
}

function read(cache: InMemoryCache) {
  return cache.readQuery({ query: MyCardgroupsConnectionDocument, variables: VARS });
}

describe("prependMyCardgroupEdge", () => {
  it("prepends a new edge and bumps totalCount on a warm cache", () => {
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a", "b"], 5));

    prependMyCardgroupEdge(cache, makeNode("new"));

    const conn = read(cache)?.myCardgroupsConnection;
    expect(conn?.edges.map((e) => e.node.id)).toEqual(["new", "a", "b"]);
    expect(conn?.totalCount).toBe(6);
  });

  it("seeds a fresh connection on a cold cache", () => {
    const cache = new InMemoryCache();

    prependMyCardgroupEdge(cache, makeNode("new"));

    const conn = read(cache)?.myCardgroupsConnection;
    expect(conn?.edges.map((e) => e.node.id)).toEqual(["new"]);
    expect(conn?.totalCount).toBe(1);
  });
});

describe("removeMyCardgroupEdge", () => {
  it("filters the edge, decrements totalCount, and returns the snapshot", () => {
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a", "b", "c"], 3));

    const snapshot = removeMyCardgroupEdge(cache, "b", VARS);

    // Snapshot reflects the pre-removal state.
    expect(snapshot?.myCardgroupsConnection.edges.map((e) => e.node.id)).toEqual(["a", "b", "c"]);
    expect(snapshot?.myCardgroupsConnection.totalCount).toBe(3);

    // The live cache has the edge removed and totalCount decremented.
    const conn = read(cache)?.myCardgroupsConnection;
    expect(conn?.edges.map((e) => e.node.id)).toEqual(["a", "c"]);
    expect(conn?.totalCount).toBe(2);
  });

  it("clamps totalCount at 0", () => {
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a"], 0));

    removeMyCardgroupEdge(cache, "a", VARS);

    expect(read(cache)?.myCardgroupsConnection.totalCount).toBe(0);
  });

  it("does not evict the normalized entity (undo window keeps it live)", () => {
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a", "b"], 2));

    removeMyCardgroupEdge(cache, "a", VARS);

    // The Cardgroup:a entity is still readable from the normalized cache.
    expect(cache.extract()["Cardgroup:a"]).toBeDefined();
  });

  it("returns null on a cold-cache miss and writes nothing", () => {
    const cache = new InMemoryCache();

    const snapshot = removeMyCardgroupEdge(cache, "a", VARS);

    expect(snapshot).toBeNull();
    expect(read(cache)).toBeNull();
  });
});

describe("restoreMyCardgroupSnapshot", () => {
  it("re-inserts the removed edge from a captured snapshot", () => {
    const cache = new InMemoryCache();
    seed(cache, makeConnection(["a", "b", "c"], 3));

    const snapshot = removeMyCardgroupEdge(cache, "b", VARS);
    expect(read(cache)?.myCardgroupsConnection.edges.map((e) => e.node.id)).toEqual(["a", "c"]);

    // biome-ignore lint/style/noNonNullAssertion: snapshot is non-null on a warm cache.
    restoreMyCardgroupSnapshot(cache, snapshot!, VARS);

    const conn = read(cache)?.myCardgroupsConnection;
    expect(conn?.edges.map((e) => e.node.id)).toEqual(["a", "b", "c"]);
    expect(conn?.totalCount).toBe(3);
  });
});
