// @vitest-environment happy-dom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  MyCardgroupsConnectionDocument,
  type MyCardgroupsConnectionQuery,
  type MyCardgroupsConnectionQueryVariables,
} from "@/generated/graphql";
import { useSeedConnectionCache } from "./use-seed-connection-cache";

const DEFAULT_VARS: MyCardgroupsConnectionQueryVariables = { first: 20, search: null };

type CgConnection = MyCardgroupsConnectionQuery["myCardgroupsConnection"];

// A complete connection slice (every field the query selects) so a cache
// round-trip via readQuery reconstructs an equal object.
function makeConnection(): CgConnection {
  return {
    __typename: "CardgroupConnection",
    edges: [
      {
        __typename: "CardgroupEdge",
        cursor: "cg-1",
        node: {
          __typename: "Cardgroup",
          id: "cg-1",
          name: "Alpha",
          updatedAt: "2024-06-15T10:00:00.000Z",
        },
      },
    ],
    pageInfo: {
      __typename: "PageInfo",
      hasNextPage: false,
      hasPreviousPage: false,
      startCursor: "cg-1",
      endCursor: "cg-1",
    },
    totalCount: 1,
  };
}

function wrapperFor(cache: InMemoryCache) {
  return ({ children }: { children: ReactNode }) =>
    createElement(MockedProvider, { cache, mocks: [] }, children);
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSeedConnectionCache", () => {
  it("seeds the connection into the cache once", () => {
    const cache = new InMemoryCache();
    const writeSpy = vi.spyOn(cache, "writeQuery");
    const connection = makeConnection();

    renderHook(
      () =>
        useSeedConnectionCache({
          document: MyCardgroupsConnectionDocument,
          variables: DEFAULT_VARS,
          data: { myCardgroupsConnection: connection },
        }),
      { wrapper: wrapperFor(cache) },
    );

    expect(writeSpy).toHaveBeenCalledTimes(1);
    expect(
      cache.readQuery({ query: MyCardgroupsConnectionDocument, variables: DEFAULT_VARS }),
    ).toEqual({ myCardgroupsConnection: connection });
  });

  it("is idempotent across re-renders — exactly one write survives", () => {
    const cache = new InMemoryCache();
    const writeSpy = vi.spyOn(cache, "writeQuery");

    const { rerender } = renderHook(
      () =>
        useSeedConnectionCache({
          document: MyCardgroupsConnectionDocument,
          variables: DEFAULT_VARS,
          data: { myCardgroupsConnection: makeConnection() },
        }),
      { wrapper: wrapperFor(cache) },
    );

    rerender();
    rerender();

    expect(writeSpy).toHaveBeenCalledTimes(1);
  });

  it("warns and skips the seed when the connection payload is missing and warnScope is set", () => {
    const cache = new InMemoryCache();
    const writeSpy = vi.spyOn(cache, "writeQuery");
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

    renderHook(
      () =>
        useSeedConnectionCache({
          document: MyCardgroupsConnectionDocument,
          variables: DEFAULT_VARS,
          data: { myCardgroupsConnection: null },
          warnScope: "cardgroups-client",
        }),
      { wrapper: wrapperFor(cache) },
    );

    expect(writeSpy).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("[cardgroups-client] initialConnection is null"),
    );
  });

  it("skips silently when the payload is missing and no warnScope is given", () => {
    const cache = new InMemoryCache();
    const writeSpy = vi.spyOn(cache, "writeQuery");
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

    renderHook(
      () =>
        useSeedConnectionCache({
          document: MyCardgroupsConnectionDocument,
          variables: DEFAULT_VARS,
          data: { myCardgroupsConnection: null },
        }),
      { wrapper: wrapperFor(cache) },
    );

    expect(writeSpy).not.toHaveBeenCalled();
    expect(warnSpy).not.toHaveBeenCalled();
  });
});
