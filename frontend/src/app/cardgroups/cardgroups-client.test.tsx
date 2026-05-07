// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MyCardgroupsConnectionDocument, MyCardgroupsDocument } from "@/generated/graphql";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "../../../__tests__/utils/mock-apollo-paginated";
import CardgroupsClient from "./cardgroups-client";
import { CARDGROUPS_DEFAULT_VARS, CARDGROUPS_PAGE_SIZE } from "./queries";

// ---------------------------------------------------------------------------
// Stub next/navigation and next/link
// ---------------------------------------------------------------------------

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn() }),
  redirect: vi.fn((path: string) => {
    throw new Error(`REDIRECT:${path}`);
  }),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...rest
  }: {
    href: string;
    children: React.ReactNode;
    [key: string]: unknown;
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const CG_1 = {
  __typename: "Cardgroup" as const,
  id: "cg-1",
  name: "Spanish Vocab",
  updatedAt: "2024-06-15T10:00:00.000Z",
};

const CG_2 = {
  __typename: "Cardgroup" as const,
  id: "cg-2",
  name: "Math Formulas",
  updatedAt: "2024-05-20T08:00:00.000Z",
};

const CG_3 = {
  __typename: "Cardgroup" as const,
  id: "cg-3",
  name: "Biology Notes",
  updatedAt: "2024-04-10T06:00:00.000Z",
};

function cgEdge(cg: typeof CG_1) {
  return {
    __typename: "CardgroupEdge" as const,
    cursor: cg.id,
    node: cg,
  };
}

function makeConnection(items: (typeof CG_1)[], hasNextPage = false, totalCount?: number) {
  return {
    __typename: "CardgroupConnection" as const,
    edges: items.map(cgEdge),
    pageInfo: {
      __typename: "PageInfo" as const,
      hasNextPage,
      hasPreviousPage: false,
      startCursor: items[0]?.id ?? null,
      endCursor: items[items.length - 1]?.id ?? null,
    },
    totalCount: totalCount ?? items.length,
  };
}

// ---------------------------------------------------------------------------
// IntersectionObserver stub
// ---------------------------------------------------------------------------

// Holds the callback for the most recently created observer so tests can
// trigger intersection events.
let ioCallbacks: IntersectionObserverCallback[] = [];

class FakeIntersectionObserver {
  constructor(cb: IntersectionObserverCallback) {
    ioCallbacks.push(cb);
  }
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
}

function fireIntersect() {
  const cb = ioCallbacks[ioCallbacks.length - 1];
  if (!cb) return;
  cb([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver);
}

// ---------------------------------------------------------------------------
// Lifecycle — leak spy + stub IntersectionObserver
// ---------------------------------------------------------------------------

let leakSpy: ApolloMockLeakSpyResult;

beforeEach(() => {
  // pagination.md: installApolloMockLeakSpy in beforeEach
  leakSpy = installApolloMockLeakSpy({ operationNames: ["MyCardgroupsConnection"] });
  ioCallbacks = [];
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  // LIFO per pagination.md Spy stacking rule: assert + teardown the leak spy last
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
  leakSpy.assertNoLeaks();
  leakSpy.teardown();
});

// ---------------------------------------------------------------------------
// Render helper
// ---------------------------------------------------------------------------

function renderClient(
  mocks: unknown[],
  initialConnection: ReturnType<typeof makeConnection> | null = null,
  cache?: InMemoryCache,
) {
  render(
    <MockedProvider mocks={mocks as never} cache={cache}>
      <CardgroupsClient initialConnection={initialConnection} />
    </MockedProvider>,
  );
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

describe("<CardgroupsClient>", () => {
  // "New cardgroup" header button is hidden on mobile because GlobalFAB
  // provides the same action at <md breakpoints. Mirrors the
  // `learn-add-card-floating` PC-only pattern.
  it("hides the 'New cardgroup' header button below md (mobile uses GlobalFAB)", async () => {
    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };

    renderClient([initialMock], null, cache);

    const link = await screen.findByRole("link", { name: /new cardgroup/i });
    expect(link.className).toContain("hidden");
    expect(link.className).toContain("md:inline-flex");
  });

  // S1: empty state — no cardgroups, no active search
  it("renders empty state when no cardgroups and no search", async () => {
    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };

    renderClient([initialMock], null, cache);

    expect(await screen.findByTestId("cardgroups-empty")).toBeInTheDocument();
    expect(screen.getByText("No cardgroups yet")).toBeInTheDocument();
    expect(screen.queryByTestId("cardgroups-list")).not.toBeInTheDocument();
  });

  // S4: renders list when query returns edges
  it("renders cardgroup list when query returns edges", async () => {
    const cache = new InMemoryCache();
    const conn = makeConnection([CG_1, CG_2]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: conn },
    });

    const listMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: conn } },
    };

    renderClient([listMock], null, cache);

    expect(await screen.findByTestId("cardgroups-list")).toBeInTheDocument();
    expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
    expect(screen.getByText("Math Formulas")).toBeInTheDocument();
  });

  // S2: no-hits state when search yields zero matches
  it("renders no-hits state when search yields zero matches", async () => {
    const user = userEvent.setup({ delay: null });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };
    const searchMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: "nonexistent" },
      },
      result: { data: { myCardgroupsConnection: makeConnection([]) } },
    };

    renderClient([initialMock, searchMock], null, cache);

    // Wait for initial empty state
    expect(await screen.findByTestId("cardgroups-empty")).toBeInTheDocument();

    const searchInput = screen.getByRole("searchbox");
    await user.type(searchInput, "nonexistent");

    // Advance past the 300ms debounce
    vi.advanceTimersByTime(300);
    vi.useRealTimers();

    await waitFor(() => {
      expect(screen.getByTestId("cardgroups-empty-search")).toBeInTheDocument();
    });
    expect(screen.getByText(/No cardgroups match "nonexistent"/)).toBeInTheDocument();
  });

  // S3: debounces search input — query fires only after 300ms
  it("debounces search input by 300ms", async () => {
    const user = userEvent.setup({ delay: null });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const cache = new InMemoryCache();
    const emptyConn = makeConnection([]);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: emptyConn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: emptyConn } },
    };
    let queryCalled = false;
    const searchMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: "vocab" },
      },
      result: () => {
        queryCalled = true;
        return { data: { myCardgroupsConnection: makeConnection([CG_1]) } };
      },
    };

    renderClient([initialMock, searchMock], null, cache);

    // Wait for initial render
    expect(await screen.findByTestId("cardgroups-empty")).toBeInTheDocument();

    const searchInput = screen.getByRole("searchbox");
    await user.type(searchInput, "vocab");

    // Should not have fired yet — still within debounce window
    expect(queryCalled).toBe(false);

    // Advance past the 300ms debounce
    vi.advanceTimersByTime(300);
    vi.useRealTimers();

    await waitFor(() => {
      expect(queryCalled).toBe(true);
    });
  });

  // S5: fetches next page when sentinel intersects (forward pagination)
  it("fetches next page when sentinel intersects", async () => {
    const cache = new InMemoryCache();
    const page1Conn = makeConnection([CG_1, CG_2], true, 3);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: page1Conn },
    });

    // Initial query mock (may be served from cache, kept for completeness)
    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: page1Conn } },
    };

    // fetchMore mock with after cursor
    const page2Mock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: {
          first: CARDGROUPS_PAGE_SIZE,
          after: CG_2.id,
          search: null,
        },
      },
      result: {
        data: {
          myCardgroupsConnection: makeConnection([CG_3], false, 3),
        },
      },
    };

    renderClient([initialMock, page2Mock], null, cache);

    // Wait for page 1 to load
    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();
    expect(screen.getByText("Math Formulas")).toBeInTheDocument();

    // Simulate the sentinel entering the viewport
    fireIntersect();

    // Page 2 item should appear after fetchMore resolves
    await waitFor(() => {
      expect(screen.getByText("Biology Notes")).toBeInTheDocument();
    });
  });

  // S5b: in-flight guard — fireIntersect called twice in the same tick fires
  //      fetchMore only once. The second call must be swallowed by fetchingRef.
  //      The LeakSpy detects any leaked second fetchMore request.
  it("does not fire fetchMore twice when sentinel intersects in the same animation frame", async () => {
    const cache = new InMemoryCache();
    const page1Conn = makeConnection([CG_1, CG_2], true, 3);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: page1Conn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: page1Conn } },
    };

    let nextPageCalls = 0;
    // Only ONE nextPageMock entry — a second fetchMore would be an unmatched
    // request and the LeakSpy in afterEach converts the console.warn into a
    // hard test failure.
    const nextPageMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: {
          first: CARDGROUPS_PAGE_SIZE,
          after: CG_2.id,
          search: null,
        },
      },
      result: () => {
        nextPageCalls += 1;
        return { data: { myCardgroupsConnection: makeConnection([CG_3], false, 3) } };
      },
    };

    renderClient([initialMock, nextPageMock], null, cache);

    // Wait for page 1 to render.
    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();

    // Fire the sentinel intersection **twice synchronously in the same tick**.
    // The useRef<boolean> in-flight guard must swallow the second call before it
    // reaches fetchMore. If it does not, the LeakSpy in afterEach will catch the
    // unmatched second request and fail the test.
    fireIntersect();
    fireIntersect();

    // Wait for the single page-2 fetch to complete.
    await waitFor(() => {
      expect(screen.getByText("Biology Notes")).toBeInTheDocument();
    });

    // The production fetchMore must have been called exactly once.
    expect(nextPageCalls).toBe(1);
  });

  // S6: halts IO loop on fetchMore error, shows retry, succeeds after retry
  //
  // Two MockedResponse entries for fetchMore per pagination.md:
  // first is an error, second is success for the retry.
  it("halts IO loop on fetchMore error and shows retry banner, succeeds after retry", async () => {
    const cache = new InMemoryCache();
    const page1Conn = makeConnection([CG_1, CG_2], true, 3);
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      data: { myCardgroupsConnection: page1Conn },
    });

    const initialMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: { first: CARDGROUPS_PAGE_SIZE, search: null },
      },
      result: { data: { myCardgroupsConnection: page1Conn } },
    };

    const fetchMoreVars = {
      first: CARDGROUPS_PAGE_SIZE,
      after: CG_2.id,
      search: null,
    };

    // First attempt fails
    const errorMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: fetchMoreVars,
      },
      result: {
        errors: [
          new GraphQLError("Could not load more", {
            extensions: { code: "INTERNAL" },
          }),
        ],
      },
    };
    // Retry succeeds
    const retryMock = {
      request: {
        query: MyCardgroupsConnectionDocument,
        variables: fetchMoreVars,
      },
      result: {
        data: {
          myCardgroupsConnection: makeConnection([CG_3], false, 3),
        },
      },
    };

    renderClient([initialMock, errorMock, retryMock], null, cache);

    // Wait for page 1 to load
    expect(await screen.findByText("Spanish Vocab")).toBeInTheDocument();

    // Trigger the first fetchMore — it will fail
    fireIntersect();

    // Error banner should appear
    await waitFor(() => {
      expect(screen.getByTestId("cardgroups-fetch-more-error")).toBeInTheDocument();
    });

    // The user clicks Retry to resume
    const retryBtn = screen.getByRole("button", { name: /retry/i });
    await userEvent.click(retryBtn);

    // Success: page 2 item appears
    await waitFor(() => {
      expect(screen.getByText("Biology Notes")).toBeInTheDocument();
    });

    // Error banner should be gone
    expect(screen.queryByTestId("cardgroups-fetch-more-error")).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// S7: connection cache update — readQuery + writeQuery (not cache.modify)
// ---------------------------------------------------------------------------
//
// These pure InMemoryCache tests verify that the update logic in
// new-cardgroup-client.tsx uses readQuery + writeQuery per pagination.md.
// cache.modify skips non-existent fields on cold cache; readQuery + writeQuery
// handles both warm and cold paths correctly.

describe("<CardgroupsClient> connection cache update (readQuery + writeQuery)", () => {
  it("prepends new cardgroup into MyCardgroupsConnection cache using readQuery + writeQuery", () => {
    const cache = new InMemoryCache();

    // Pre-seed both the connection cache and the flat list cache.
    // CARDGROUPS_DEFAULT_VARS keeps the cache key identical to what
    // new-cardgroup-client.tsx and the SSR seed write — any mismatch
    // would make this read invisible (cache miss).
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: { myCardgroupsConnection: makeConnection([CG_1, CG_2]) },
    });
    cache.writeQuery({
      query: MyCardgroupsDocument,
      data: { myCardgroups: [CG_1, CG_2] },
    });

    const NEW_CG = {
      __typename: "Cardgroup" as const,
      id: "cg-new",
      name: "New Cardgroup",
      updatedAt: "2026-05-01T00:00:00.000Z",
    };

    // Warm-cache path: readQuery returns data, writeQuery prepends
    const existingConnection = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });

    // Verify readQuery returned non-null (warm cache)
    expect(existingConnection).not.toBeNull();

    if (existingConnection) {
      cache.writeQuery({
        query: MyCardgroupsConnectionDocument,
        variables: CARDGROUPS_DEFAULT_VARS,
        data: {
          myCardgroupsConnection: {
            ...existingConnection.myCardgroupsConnection,
            edges: [
              { __typename: "CardgroupEdge" as const, cursor: NEW_CG.id, node: NEW_CG },
              ...existingConnection.myCardgroupsConnection.edges,
            ],
            totalCount: existingConnection.myCardgroupsConnection.totalCount + 1,
          },
        },
      });
    }

    const result = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });

    expect(result?.myCardgroupsConnection.edges).toHaveLength(3);
    expect(result?.myCardgroupsConnection.edges[0]).toMatchObject({
      cursor: NEW_CG.id,
      node: { id: NEW_CG.id, name: NEW_CG.name },
    });
    expect(result?.myCardgroupsConnection.totalCount).toBe(3);
  });

  it("builds minimal connection on cold cache (readQuery returns null)", () => {
    const cache = new InMemoryCache();
    // Do NOT seed — cold-cache scenario

    const NEW_CG = {
      __typename: "Cardgroup" as const,
      id: "cg-new",
      name: "New Cardgroup",
      updatedAt: "2026-05-01T00:00:00.000Z",
    };

    const existingConnection = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });

    // Cold cache: readQuery must return null
    expect(existingConnection).toBeNull();

    // Build a minimal connection (as new-cardgroup-client.tsx does in the else branch)
    cache.writeQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
      data: {
        myCardgroupsConnection: {
          __typename: "CardgroupConnection" as const,
          edges: [{ __typename: "CardgroupEdge" as const, cursor: NEW_CG.id, node: NEW_CG }],
          pageInfo: {
            __typename: "PageInfo" as const,
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: NEW_CG.id,
            endCursor: NEW_CG.id,
          },
          totalCount: 1,
        },
      },
    });

    const result = cache.readQuery({
      query: MyCardgroupsConnectionDocument,
      variables: CARDGROUPS_DEFAULT_VARS,
    });

    expect(result?.myCardgroupsConnection.edges).toHaveLength(1);
    expect(result?.myCardgroupsConnection.edges[0]).toMatchObject({
      cursor: NEW_CG.id,
      node: { id: NEW_CG.id, name: NEW_CG.name },
    });
    expect(result?.myCardgroupsConnection.pageInfo.hasNextPage).toBe(false);
    expect(result?.myCardgroupsConnection.totalCount).toBe(1);
  });
});
