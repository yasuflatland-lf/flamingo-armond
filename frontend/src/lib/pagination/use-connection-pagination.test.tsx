// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, render, waitFor } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { createElement, type ReactNode, useMemo } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  MyCardgroupsConnectionDocument,
  type MyCardgroupsConnectionQuery,
  type MyCardgroupsConnectionQueryVariables,
} from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../__tests__/utils/mock-apollo-paginated";
import {
  type UseConnectionPaginationResult,
  useConnectionPagination,
} from "./use-connection-pagination";

// ---------------------------------------------------------------------------
// This suite exercises the GENERIC hook against a non-cards document
// (MyCardgroupsConnection) to prove the generalisation is document-agnostic.
// The cards-specific behaviour is covered separately by
// app/cardgroups/[id]/cards/use-cards-connection.test.ts (the regression proof
// that the wrapper is behaviour-preserving).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const PAGE_SIZE = 20;
const DEFAULT_VARS: MyCardgroupsConnectionQueryVariables = { first: PAGE_SIZE, search: null };

const CG_1 = {
  __typename: "Cardgroup" as const,
  id: "cg-1",
  name: "Alpha",
  updatedAt: "2024-06-15T10:00:00.000Z",
};
const CG_2 = {
  __typename: "Cardgroup" as const,
  id: "cg-2",
  name: "Beta",
  updatedAt: "2024-05-20T08:00:00.000Z",
};
const CG_3 = {
  __typename: "Cardgroup" as const,
  id: "cg-3",
  name: "Gamma",
  updatedAt: "2024-04-10T06:00:00.000Z",
};

type CgLike = typeof CG_1;
type CgConnection = MyCardgroupsConnectionQuery["myCardgroupsConnection"];
type CgEdge = CgConnection["edges"][number];
type CgPageInfo = CgConnection["pageInfo"];

const EMPTY_PAGE_INFO: CgPageInfo = {
  __typename: "PageInfo",
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: null,
  endCursor: null,
};

function cgEdge(cg: CgLike): CgEdge {
  return { __typename: "CardgroupEdge", cursor: cg.id, node: cg };
}

function connection(items: ReadonlyArray<CgLike>, hasNext = false): CgConnection {
  return {
    __typename: "CardgroupConnection",
    edges: items.map(cgEdge),
    pageInfo: {
      __typename: "PageInfo",
      hasNextPage: hasNext,
      hasPreviousPage: false,
      startCursor: items[0]?.id ?? null,
      endCursor: items[items.length - 1]?.id ?? null,
    },
    totalCount: items.length,
  };
}

// ---------------------------------------------------------------------------
// Caller-supplied wiring (the three cards-specific bits, now injected).
// ---------------------------------------------------------------------------

const selectConnection = (data: MyCardgroupsConnectionQuery | undefined) =>
  data?.myCardgroupsConnection;

const buildFetchMoreVariables = (
  after: string | null,
  search: string | null,
): MyCardgroupsConnectionQueryVariables => ({ ...DEFAULT_VARS, after, search });

const mergeConnection = (
  prev: MyCardgroupsConnectionQuery,
  more: MyCardgroupsConnectionQuery,
): MyCardgroupsConnectionQuery => ({
  myCardgroupsConnection: {
    ...more.myCardgroupsConnection,
    edges: [...prev.myCardgroupsConnection.edges, ...more.myCardgroupsConnection.edges],
  },
});

const resolveFetchMoreError = () => "Could not load more.";

// ---------------------------------------------------------------------------
// IntersectionObserver stub — capture observer instances so each test can
// invoke their callbacks and assert observe/disconnect lifecycle.
// ---------------------------------------------------------------------------

type FakeObserver = {
  cb: IntersectionObserverCallback;
  observed: Element[];
  disconnected: boolean;
};

let observers: FakeObserver[] = [];

class FakeIntersectionObserver {
  private state: FakeObserver;
  constructor(cb: IntersectionObserverCallback) {
    this.state = { cb, observed: [], disconnected: false };
    observers.push(this.state);
  }
  observe(node: Element) {
    this.state.observed.push(node);
  }
  unobserve() {}
  disconnect() {
    this.state.disconnected = true;
  }
  takeRecords() {
    return [];
  }
}

function latestObserver(): FakeObserver | undefined {
  for (let i = observers.length - 1; i >= 0; i -= 1) {
    const o = observers[i];
    if (o && !o.disconnected) return o;
  }
  return undefined;
}

function fireIntersect() {
  const obs = latestObserver();
  if (!obs) return;
  obs.cb([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver);
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  leakSpy = installApolloMockLeakSpy({ operationNames: ["MyCardgroupsConnection"] });
  observers = [];
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

// ---------------------------------------------------------------------------
// Harness — render (not renderHook) so sentinelRef attaches to a real node
// and the IO observer effect (which short-circuits on `if (!node) return`)
// actually wires up.
// ---------------------------------------------------------------------------

type HookResult = UseConnectionPaginationResult<
  MyCardgroupsConnectionQuery,
  CgEdge,
  CgPageInfo,
  MyCardgroupsConnectionQueryVariables
>;

type ProbeProps = {
  searchQuery: string | null;
  initial: { edges: CgEdge[]; pageInfo: CgPageInfo; totalCount: number };
};

type Holder = { current: HookResult | null };

function HookProbe({ props, holder }: { props: ProbeProps; holder: Holder }) {
  const variables: MyCardgroupsConnectionQueryVariables = useMemo(
    () =>
      props.searchQuery === null ? DEFAULT_VARS : { ...DEFAULT_VARS, search: props.searchQuery },
    [props.searchQuery],
  );

  const value = useConnectionPagination<
    MyCardgroupsConnectionQuery,
    MyCardgroupsConnectionQueryVariables,
    CgEdge,
    CgPageInfo
  >({
    document: MyCardgroupsConnectionDocument,
    variables,
    searchQuery: props.searchQuery,
    selectConnection,
    buildFetchMoreVariables,
    mergeConnection,
    initial: props.initial,
    resolveFetchMoreError,
    logScope: "[test]",
  });

  holder.current = value;
  return createElement("div", { ref: value.sentinelRef, "data-testid": "sentinel" });
}

type RenderOptions = {
  mocks: ReadonlyArray<unknown>;
  cache: InMemoryCache;
  searchQuery?: string | null;
  initial?: { edges: CgEdge[]; pageInfo: CgPageInfo; totalCount: number };
};

function renderProbe(opts: RenderOptions) {
  const holder: Holder = { current: null };
  const initial = opts.initial ?? { edges: [], pageInfo: EMPTY_PAGE_INFO, totalCount: 0 };

  const wrap = (searchQuery: string | null): ReactNode =>
    createElement(
      MockedProvider,
      { mocks: opts.mocks as never, cache: opts.cache } as never,
      createElement(HookProbe, { props: { searchQuery, initial }, holder }),
    );

  const utils = render(wrap(opts.searchQuery ?? null));

  const result = {
    get current(): HookResult {
      if (!holder.current) throw new Error("useConnectionPagination has not produced a value yet");
      return holder.current;
    },
  };

  const rerender = (searchQuery: string | null) => utils.rerender(wrap(searchQuery));

  return { ...utils, result, rerender };
}

function seedCache(cache: InMemoryCache, conn: CgConnection) {
  cache.writeQuery({
    query: MyCardgroupsConnectionDocument,
    variables: DEFAULT_VARS,
    data: { myCardgroupsConnection: conn },
  });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useConnectionPagination", () => {
  it("returns the SSR-seeded edges/pageInfo/totalCount from cache without a network call", () => {
    const cache = new InMemoryCache();
    seedCache(cache, connection([CG_1, CG_2]));

    // initial is empty on purpose: a non-empty edges result proves the value
    // came from the cache seed, not from the `initial` fallback.
    const { result } = renderProbe({ mocks: [], cache });

    expect(result.current.edges).toHaveLength(2);
    expect(result.current.edges[0]?.node.id).toBe(CG_1.id);
    expect(result.current.edges[1]?.node.id).toBe(CG_2.id);
    expect(result.current.totalCount).toBe(2);
    expect(result.current.pageInfo.hasNextPage).toBe(false);
    expect(result.current.fetchMoreError).toBeNull();
    expect(result.current.queryVariables).toEqual(DEFAULT_VARS);
  });

  it("observer-driven fetchMore advances the cursor and appends the next page", async () => {
    const cache = new InMemoryCache();
    const page1 = connection([CG_1, CG_2], true);
    seedCache(cache, page1);

    const page2Mock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CG_2.id, search: null },
      },
      result: { data: { myCardgroupsConnection: connection([CG_3]) } },
    };

    const { result } = renderProbe({ mocks: [page2Mock], cache });

    expect(result.current.pageInfo.hasNextPage).toBe(true);
    expect(result.current.edges).toHaveLength(2);

    act(() => {
      fireIntersect();
    });

    await waitFor(() => {
      expect(result.current.edges).toHaveLength(3);
    });
    expect(result.current.edges[2]?.node.id).toBe(CG_3.id);
    expect(result.current.fetchMoreError).toBeNull();
  });

  it("in-flight guard prevents a second fetchMore while the first is in flight", async () => {
    let page2CallCount = 0;
    const cache = new InMemoryCache();
    seedCache(cache, connection([CG_1, CG_2], true));

    const page2Mock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CG_2.id, search: null },
      },
      delay: 30,
      result: () => {
        page2CallCount += 1;
        return { data: { myCardgroupsConnection: connection([CG_3]) } };
      },
    };

    const { result } = renderProbe({ mocks: [page2Mock], cache });

    act(() => {
      fireIntersect();
      fireIntersect();
    });

    await waitFor(() => {
      expect(result.current.edges).toHaveLength(3);
    });

    expect(page2CallCount).toBe(1);
  });

  it("fetchMore error halts the IO loop, then retryFetchMore clears it and succeeds", async () => {
    const cache = new InMemoryCache();
    seedCache(cache, connection([CG_1, CG_2], true));

    const errorMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CG_2.id, search: null },
      },
      result: {
        errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })],
      },
    };
    const retryMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CG_2.id, search: null },
      },
      result: { data: { myCardgroupsConnection: connection([CG_3]) } },
    };

    const { result } = renderProbe({ mocks: [errorMock, retryMock], cache });

    act(() => {
      fireIntersect();
    });

    await waitFor(() => {
      expect(result.current.fetchMoreError).not.toBeNull();
    });
    expect(result.current.fetchMoreError).toBe("Could not load more.");

    // Halt gate: the observer disconnected, so a second intersect does nothing.
    expect(latestObserver()).toBeUndefined();
    act(() => {
      fireIntersect();
    });
    await new Promise((r) => setTimeout(r, 10));
    expect(result.current.edges).toHaveLength(2);

    // Retry clears the error and re-runs the request, merging page 2.
    act(() => {
      result.current.retryFetchMore();
    });

    await waitFor(() => {
      expect(result.current.edges).toHaveLength(3);
    });
    expect(result.current.fetchMoreError).toBeNull();
    expect(result.current.edges[2]?.node.id).toBe(CG_3.id);
  });

  it("searchQuery change clears fetchMoreError via the immediate-reset effect", async () => {
    const cache = new InMemoryCache();
    const page1 = connection([CG_1, CG_2], true);
    seedCache(cache, page1);

    const errorMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CG_2.id, search: null },
      },
      result: {
        errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })],
      },
    };
    const searchMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { ...DEFAULT_VARS, search: "x" },
      },
      result: { data: { myCardgroupsConnection: connection([CG_1]) } },
    };

    const { result, rerender } = renderProbe({
      mocks: [errorMock, searchMock],
      cache,
      searchQuery: null,
    });

    act(() => {
      fireIntersect();
    });

    await waitFor(() => {
      expect(result.current.fetchMoreError).not.toBeNull();
    });

    rerender("x");

    await waitFor(() => {
      expect(result.current.fetchMoreError).toBeNull();
    });
    expect(result.current.queryVariables).toEqual({ ...DEFAULT_VARS, search: "x" });
  });
});
