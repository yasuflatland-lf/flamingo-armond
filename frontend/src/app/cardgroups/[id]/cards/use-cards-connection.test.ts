// @vitest-environment jsdom
import { InMemoryCache, NetworkStatus } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, render, waitFor } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { createElement, type ReactNode, useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
} from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../../../__tests__/utils/mock-apollo-paginated";
import { type UseCardsConnectionResult, useCardsConnection } from "./use-cards-connection";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const CG_ID = "cg-1";
const PAGE_SIZE = 20;

const userCardState = (due: string, state: number) => ({
  __typename: "UserCardState" as const,
  due,
  state,
});

const CARD_1 = {
  __typename: "Card" as const,
  id: "c-1",
  front: "Hello",
  back: "Hola",
  userCardState: userCardState("2024-06-15", 0),
  cardgroupId: CG_ID,
};

const CARD_2 = {
  __typename: "Card" as const,
  id: "c-2",
  front: "Bye",
  back: "Adios",
  userCardState: userCardState("2024-06-15", 0),
  cardgroupId: CG_ID,
};

const CARD_3 = {
  __typename: "Card" as const,
  id: "c-3",
  front: "Three",
  back: "Tres",
  userCardState: userCardState("2024-06-15", 0),
  cardgroupId: CG_ID,
};

type CardLike = typeof CARD_1;

type CardConnectionData = CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"];

function edge(card: CardLike) {
  return {
    __typename: "CardEdge" as const,
    cursor: card.id,
    node: card,
  };
}

function connection(cards: ReadonlyArray<CardLike>, hasNext = false): CardConnectionData {
  return {
    __typename: "CardConnection" as const,
    edges: cards.map(edge),
    pageInfo: {
      __typename: "PageInfo" as const,
      hasNextPage: hasNext,
      hasPreviousPage: false,
      startCursor: cards[0]?.id ?? null,
      endCursor: cards[cards.length - 1]?.id ?? null,
    },
    totalCount: cards.length,
  };
}

const DEFAULT_VARS = { cardgroupId: CG_ID, first: PAGE_SIZE, search: null };

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
  // Find the most recent observer that is still connected.
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
let warnSpy: ReturnType<typeof vi.spyOn> | undefined;

beforeEach(() => {
  leakSpy = installApolloMockLeakSpy({ operationNames: ["CardsByCardgroupConnection"] });
  observers = [];
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  warnSpy?.mockRestore();
  warnSpy = undefined;
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

// ---------------------------------------------------------------------------
// Harness — uses `render` (not `renderHook`) so the hook's sentinelRef can be
// attached to a real DOM node. The IO observer effect short-circuits on
// `if (!node) return`, so a bare renderHook leaves the observer un-attached
// and `fireIntersect` becomes a no-op. The harness publishes the hook's
// return value to the caller via a mutable holder updated each render.
// ---------------------------------------------------------------------------

type HookProps = {
  cardgroupId: string;
  searchQuery: string | null;
  initialEdges: ReturnType<typeof connection>["edges"];
  initialPageInfo: ReturnType<typeof connection>["pageInfo"];
  initialTotalCount: number;
};

type HarnessOptions = {
  mocks: ReadonlyArray<unknown>;
  cache?: InMemoryCache;
  seedCards?: ReadonlyArray<CardLike>;
  seedHasNext?: boolean;
  initialSearchQuery?: string | null;
};

type HookHolder = {
  current: UseCardsConnectionResult | null;
};

function HookProbe({ props, holder }: { props: HookProps; holder: HookHolder }) {
  const value = useCardsConnection(props);
  // Synchronously expose the latest value so test assertions read the freshest
  // snapshot (mirrors @testing-library/react's renderHook semantics).
  holder.current = value;
  // Touch the ref by attaching it to a real div so the IO observer's
  // useEffect picks up the sentinel node.
  useEffect(() => {
    // no-op: the ref is wired below via JSX.
  }, []);
  return createElement("div", {
    ref: value.sentinelRef,
    "data-testid": "use-cards-connection-sentinel",
  });
}

function renderUseCardsConnection(opts: HarnessOptions) {
  const seedCards = opts.seedCards ?? [CARD_1, CARD_2];
  const seed = connection(seedCards, opts.seedHasNext ?? false);

  const cache = opts.cache ?? new InMemoryCache();
  if (!opts.cache) {
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: seed },
    });
  }

  const holder: HookHolder = { current: null };

  const initialProps: HookProps = {
    cardgroupId: CG_ID,
    searchQuery: opts.initialSearchQuery ?? null,
    initialEdges: seed.edges,
    initialPageInfo: seed.pageInfo,
    initialTotalCount: seed.totalCount,
  };

  const wrap = (props: HookProps): ReactNode =>
    createElement(
      MockedProvider,
      { mocks: opts.mocks as never, cache } as never,
      createElement(HookProbe, { props, holder }),
    );

  const utils = render(wrap(initialProps));

  // Provide a `result` accessor with `.current` that always returns the
  // freshest hook value (mirroring renderHook's API).
  const result = {
    get current(): UseCardsConnectionResult {
      if (!holder.current) {
        throw new Error("useCardsConnection has not produced a value yet");
      }
      return holder.current;
    },
  };

  const rerender = (next: HookProps) => {
    utils.rerender(wrap(next));
  };

  // Order matters: spread `utils` first so our typed `rerender` (which takes
  // HookProps, not a ReactNode) wins over `utils.rerender`.
  return { ...utils, cache, seed, result, rerender };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useCardsConnection", () => {
  it("returns SSR-seeded edges, pageInfo, totalCount without a network call", () => {
    const { result } = renderUseCardsConnection({
      mocks: [],
      seedCards: [CARD_1, CARD_2],
    });

    expect(result.current.edges).toHaveLength(2);
    expect(result.current.edges[0]?.node.id).toBe(CARD_1.id);
    expect(result.current.edges[1]?.node.id).toBe(CARD_2.id);
    expect(result.current.totalCount).toBe(2);
    expect(result.current.pageInfo.hasNextPage).toBe(false);
    expect(result.current.fetchMoreError).toBeNull();
    expect(result.current.queryVariables).toEqual(DEFAULT_VARS);
  });

  it("requestNextPage advances cursor: IO trigger extends edges with page 2", async () => {
    const page1 = connection([CARD_1, CARD_2], true);
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: page1 },
    });

    const initialMock = {
      request: { query: CardsByCardgroupConnectionDocument, variables: DEFAULT_VARS },
      result: { data: { cardsByCardgroupConnection: page1 } },
    };
    const page2Mock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CARD_2.id },
      },
      result: {
        data: { cardsByCardgroupConnection: connection([CARD_3]) },
      },
    };

    const { result } = renderUseCardsConnection({
      mocks: [initialMock, page2Mock],
      cache,
      seedCards: [CARD_1, CARD_2],
      seedHasNext: true,
    });

    expect(result.current.pageInfo.hasNextPage).toBe(true);
    expect(result.current.edges).toHaveLength(2);

    act(() => {
      fireIntersect();
    });

    await waitFor(() => {
      expect(result.current.edges).toHaveLength(3);
    });
    expect(result.current.edges[2]?.node.id).toBe(CARD_3.id);
    expect(result.current.fetchMoreError).toBeNull();
  });

  it("in-flight guard prevents a second fetchMore while the first is in flight", async () => {
    let page2CallCount = 0;
    const page1 = connection([CARD_1, CARD_2], true);
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: page1 },
    });

    const initialMock = {
      request: { query: CardsByCardgroupConnectionDocument, variables: DEFAULT_VARS },
      result: { data: { cardsByCardgroupConnection: page1 } },
    };
    const page2Mock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CARD_2.id },
      },
      // delay > 0 keeps the resolution pending so the in-flight guard is active.
      delay: 30,
      result: () => {
        page2CallCount += 1;
        return { data: { cardsByCardgroupConnection: connection([CARD_3]) } };
      },
    };

    const { result } = renderUseCardsConnection({
      mocks: [initialMock, page2Mock],
      cache,
      seedCards: [CARD_1, CARD_2],
      seedHasNext: true,
    });

    // Fire twice in rapid succession — only the first should reach fetchMore.
    act(() => {
      fireIntersect();
      fireIntersect();
    });

    // Settle.
    await waitFor(() => {
      expect(result.current.edges).toHaveLength(3);
    });

    // Exactly one network call must have been issued for page 2.
    expect(page2CallCount).toBe(1);
  });

  it("fetchMoreError halts the IO loop: a second intersect does not advance", async () => {
    warnSpy = vi.spyOn(console, "warn");

    const page1 = connection([CARD_1, CARD_2], true);
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: page1 },
    });

    const initialMock = {
      request: { query: CardsByCardgroupConnectionDocument, variables: DEFAULT_VARS },
      result: { data: { cardsByCardgroupConnection: page1 } },
    };
    const failingPage2 = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CARD_2.id },
      },
      result: {
        errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })],
      },
    };

    const { result } = renderUseCardsConnection({
      mocks: [initialMock, failingPage2],
      cache,
      seedCards: [CARD_1, CARD_2],
      seedHasNext: true,
    });

    act(() => {
      fireIntersect();
    });

    await waitFor(() => {
      expect(result.current.fetchMoreError).not.toBeNull();
    });

    // The observer should now be disconnected (effect short-circuits on
    // fetchMoreError != null). The latestObserver helper returns undefined
    // because all live observers were disconnected on the cleanup pass.
    expect(latestObserver()).toBeUndefined();

    // Fire intersect again — no further network request should leak.
    act(() => {
      fireIntersect();
    });

    // Give the microtask queue time to flush any stray fetchMore.
    await new Promise((r) => setTimeout(r, 10));

    // edges must remain at the seed length (no page 2 merged).
    expect(result.current.edges).toHaveLength(2);
  });

  it("retryFetchMore clears fetchMoreError and re-runs the request to success", async () => {
    warnSpy = vi.spyOn(console, "warn");

    const page1 = connection([CARD_1, CARD_2], true);
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: page1 },
    });

    const initialMock = {
      request: { query: CardsByCardgroupConnectionDocument, variables: DEFAULT_VARS },
      result: { data: { cardsByCardgroupConnection: page1 } },
    };
    const errorMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CARD_2.id },
      },
      error: new Error("network failure"),
    };
    const retryMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CARD_2.id },
      },
      result: {
        data: { cardsByCardgroupConnection: connection([CARD_3]) },
      },
    };

    const { result } = renderUseCardsConnection({
      mocks: [initialMock, errorMock, retryMock],
      cache,
      seedCards: [CARD_1, CARD_2],
      seedHasNext: true,
    });

    act(() => {
      fireIntersect();
    });

    await waitFor(() => {
      expect(result.current.fetchMoreError).not.toBeNull();
    });

    // Retry clears the error and re-fires the request, merging the next page.
    act(() => {
      result.current.retryFetchMore();
    });

    await waitFor(() => {
      expect(result.current.edges).toHaveLength(3);
    });
    expect(result.current.fetchMoreError).toBeNull();
    expect(result.current.edges[2]?.node.id).toBe(CARD_3.id);
  });

  it("searchQuery change clears fetchMoreError via the immediate-reset effect", async () => {
    warnSpy = vi.spyOn(console, "warn");

    const page1 = connection([CARD_1, CARD_2], true);
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: page1 },
    });

    const initialMock = {
      request: { query: CardsByCardgroupConnectionDocument, variables: DEFAULT_VARS },
      result: { data: { cardsByCardgroupConnection: page1 } },
    };
    const failingPage2 = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CARD_2.id },
      },
      result: {
        errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })],
      },
    };
    const searchMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, search: "x" },
      },
      result: { data: { cardsByCardgroupConnection: connection([CARD_1]) } },
    };

    const { result, rerender } = renderUseCardsConnection({
      mocks: [initialMock, failingPage2, searchMock],
      cache,
      seedCards: [CARD_1, CARD_2],
      seedHasNext: true,
      initialSearchQuery: null,
    });

    // Fail the first fetchMore.
    act(() => {
      fireIntersect();
    });

    await waitFor(() => {
      expect(result.current.fetchMoreError).not.toBeNull();
    });

    // Flip searchQuery — the immediate-reset effect MUST clear fetchMoreError.
    rerender({
      cardgroupId: CG_ID,
      searchQuery: "x",
      initialEdges: page1.edges,
      initialPageInfo: page1.pageInfo,
      initialTotalCount: page1.totalCount,
    });

    await waitFor(() => {
      expect(result.current.fetchMoreError).toBeNull();
    });
    expect(result.current.queryVariables).toEqual({ ...DEFAULT_VARS, search: "x" });
  });

  it("fetchingMore reflects NetworkStatus.fetchMore mid-fetch and clears after", async () => {
    const page1 = connection([CARD_1, CARD_2], true);
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: DEFAULT_VARS,
      data: { cardsByCardgroupConnection: page1 },
    });

    const initialMock = {
      request: { query: CardsByCardgroupConnectionDocument, variables: DEFAULT_VARS },
      result: { data: { cardsByCardgroupConnection: page1 } },
    };
    // A long delay keeps the in-flight state observable; the wait-loop below
    // races the resolution and asserts the intermediate NetworkStatus.fetchMore.
    const page2Mock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { ...DEFAULT_VARS, after: CARD_2.id },
      },
      delay: 200,
      result: { data: { cardsByCardgroupConnection: connection([CARD_3]) } },
    };

    const { result } = renderUseCardsConnection({
      mocks: [initialMock, page2Mock],
      cache,
      seedCards: [CARD_1, CARD_2],
      seedHasNext: true,
    });

    expect(result.current.fetchingMore).toBe(false);
    expect(result.current.networkStatus).toBe(NetworkStatus.ready);

    act(() => {
      fireIntersect();
    });

    // Mid-fetch: networkStatus should flip to NetworkStatus.fetchMore once
    // Apollo announces the in-flight state via notifyOnNetworkStatusChange.
    await waitFor(() => {
      expect(result.current.networkStatus).toBe(NetworkStatus.fetchMore);
    });
    expect(result.current.fetchingMore).toBe(true);

    // After completion: edges grow, networkStatus settles back to ready,
    // fetchingMore = false.
    await waitFor(() => {
      expect(result.current.edges).toHaveLength(3);
    });
    await waitFor(() => {
      expect(result.current.fetchingMore).toBe(false);
    });
    expect(result.current.networkStatus).toBe(NetworkStatus.ready);
  });

  it("queryError is non-null when the initial query fails with a network error", async () => {
    // Use an empty cache (no SSR seed) so the query fires against the mock
    // rather than resolving from cache. The network-error mock sets the
    // useQuery `error` field, which the hook surfaces as `queryError`.
    const cache = new InMemoryCache();

    const failingMock = {
      request: { query: CardsByCardgroupConnectionDocument, variables: DEFAULT_VARS },
      error: new Error("network failure"),
    };

    const { result } = renderUseCardsConnection({
      mocks: [failingMock],
      cache,
    });

    await waitFor(() => {
      expect(result.current.queryError).toBeDefined();
    });

    expect(result.current.queryError?.message).toBeDefined();
  });
});
