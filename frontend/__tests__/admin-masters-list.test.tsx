// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { AdminMastersClient } from "@/app/admin/masters/admin-masters-client";
import { ADMIN_MASTERS_PAGE_SIZE } from "@/app/admin/masters/queries";
import { AdminMastersDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import {
  type ApolloMockLeakSpyResult,
  installApolloMockLeakSpy,
} from "./utils/mock-apollo-paginated";

// ---------------------------------------------------------------------------
// Next.js stubs. usePathname drives the real useSheetSearchParam URL state —
// the broad test exercises the real sheet hook, unlike the narrow co-located
// test which mocks useSheetSearchParam entirely.
// ---------------------------------------------------------------------------

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  usePathname: () => "/admin/masters",
  useRouter: () => ({ push: mockPush, refresh: vi.fn(), replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(""),
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

vi.mock("next/image", () => ({
  default: ({
    src,
    alt,
    width,
    height,
    ...rest
  }: {
    src: string;
    alt: string;
    width: number;
    height: number;
    [key: string]: unknown;
  }) => (
    // biome-ignore lint/performance/noImgElement: deliberate next/image stub for tests
    <img src={src} alt={alt} width={width} height={height} {...rest} />
  ),
}));

// Toasts are fired by the create/update/delete paths the broad test does not
// exercise; stub them so the row/list mutations never reach the real sonner.
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

// ---------------------------------------------------------------------------
// Data fixtures — inlined (no fixtures/masters.ts). Master GraphQL typenames:
// node MasterCardgroup, edge MasterCatalogEdge, connection MasterCatalogConnection.
// A node selects every field AdminMastersQuery requests.
// ---------------------------------------------------------------------------

type MasterNode = {
  __typename: "MasterCardgroup";
  id: string;
  name: string;
  description: string | null;
  language: string | null;
  level: string | null;
  category: string | null;
  coverImageUrl: string | null;
  source: string | null;
  version: number;
  status: "DRAFT" | "PUBLISHED";
  isDefaultStarter: boolean;
  sortOrder: number;
  cardCount: number;
};

type MasterEdge = {
  __typename: "MasterCatalogEdge";
  cursor: string;
  node: MasterNode;
};

type MasterConnection = {
  __typename: "MasterCatalogConnection";
  edges: MasterEdge[];
  pageInfo: {
    __typename: "PageInfo";
    hasNextPage: boolean;
    hasPreviousPage: boolean;
    startCursor: string | null;
    endCursor: string | null;
  };
  totalCount: number;
};

function makeMaster(i: number): MasterNode {
  return {
    __typename: "MasterCardgroup",
    id: `m-${i}`,
    name: `Deck m-${i}`,
    description: null,
    language: null,
    level: null,
    category: null,
    coverImageUrl: null,
    source: null,
    version: 1,
    status: "DRAFT",
    isDefaultStarter: false,
    sortOrder: i,
    cardCount: 5,
  };
}

function makeEdge(master: MasterNode): MasterEdge {
  return {
    __typename: "MasterCatalogEdge",
    cursor: master.id,
    node: master,
  };
}

function makeConnection(masters: MasterNode[], hasNextPage: boolean): MasterConnection {
  return {
    __typename: "MasterCatalogConnection",
    edges: masters.map(makeEdge),
    pageInfo: {
      __typename: "PageInfo",
      hasNextPage,
      hasPreviousPage: false,
      startCursor: masters[0]?.id ?? null,
      endCursor: masters[masters.length - 1]?.id ?? null,
    },
    totalCount: masters.length,
  };
}

// The component's useQuery always sends orderBy / orderDirection / first.
// `as const` narrows the enum literals to MasterCatalogOrderBy / SortOrder so
// the typed MockedProvider variables accept them.
const BASE_VARS = {
  first: ADMIN_MASTERS_PAGE_SIZE,
  orderBy: "SORT_ORDER" as const,
  orderDirection: "ASC" as const,
};

// ---------------------------------------------------------------------------
// IntersectionObserver mock — captures each callback so the test can fire the
// most-recently-created observer (the component recreates it on render).
// ---------------------------------------------------------------------------

let ioCallbacks: IntersectionObserverCallback[] = [];

class FakeIntersectionObserver {
  callback: IntersectionObserverCallback;
  constructor(cb: IntersectionObserverCallback) {
    this.callback = cb;
    ioCallbacks.push(cb);
  }
  observe() {}
  unobserve() {}
  disconnect() {
    ioCallbacks = ioCallbacks.filter((cb) => cb !== this.callback);
  }
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

function fireIntersect() {
  const cb = ioCallbacks[ioCallbacks.length - 1];
  if (!cb) return;
  cb([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver);
}

// ---------------------------------------------------------------------------
// Leak spy + lifecycle.
// ---------------------------------------------------------------------------

let leak: ApolloMockLeakSpyResult;

beforeEach(() => {
  ioCallbacks = [];
  mockPush.mockReset();
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
  vi.spyOn(console, "error").mockImplementation(() => {});
  leak = installApolloMockLeakSpy({ operationNames: ["AdminMasters"] });
});

afterEach(() => {
  leak.assertNoLeaks();
  leak.teardown();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("AdminMastersClient (broad page test)", () => {
  // T1: Initial list render — several master nodes render and the list container is present.
  test("renders the master rows and the list container after the initial query", async () => {
    const masters = Array.from({ length: 3 }, (_, i) => makeMaster(i + 1));
    const connection = makeConnection(masters, false);

    const mocks = [
      {
        request: {
          query: AdminMastersDocument,
          variables: { ...BASE_VARS, search: null },
        },
        result: { data: { adminMasters: connection } },
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminMastersDocument,
      variables: { ...BASE_VARS, search: null },
      data: { adminMasters: connection },
    });

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminMastersClient />
      </MockedProvider>,
    );

    // Rows visible.
    expect(await screen.findByText("Deck m-1")).toBeInTheDocument();
    expect(screen.getByText("Deck m-2")).toBeInTheDocument();
    expect(screen.getByText("Deck m-3")).toBeInTheDocument();

    // List container rendered.
    expect(screen.getByTestId("admin-masters-list")).toBeInTheDocument();

    // totalCount shown (3 in parens).
    expect(screen.getByText("(3)")).toBeInTheDocument();
  });

  // T2: Debounced search — typing into the search input issues a second query
  // with search:"ali" after the 300ms debounce, and the filtered result renders.
  test('search input debounces 300ms then refetches AdminMasters with search "ali"', async () => {
    const user = userEvent.setup({ delay: null });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const initialMasters = Array.from({ length: 2 }, (_, i) => makeMaster(i + 1));
    const searchMaster: MasterNode = { ...makeMaster(99), id: "m-ali", name: "Alpha Deck" };

    let searchQueryCalls = 0;
    const searchResult = vi.fn(() => {
      searchQueryCalls += 1;
      return { data: { adminMasters: makeConnection([searchMaster], false) } };
    });

    const mocks = [
      {
        request: {
          query: AdminMastersDocument,
          variables: { ...BASE_VARS, search: null },
        },
        result: { data: { adminMasters: makeConnection(initialMasters, false) } },
      },
      {
        request: {
          query: AdminMastersDocument,
          variables: { ...BASE_VARS, search: "ali" },
        },
        result: searchResult,
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminMastersDocument,
      variables: { ...BASE_VARS, search: null },
      data: { adminMasters: makeConnection(initialMasters, false) },
    });

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminMastersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("Deck m-1")).toBeInTheDocument();

    const searchInput = screen.getByRole("searchbox", { name: /search masters/i });

    // Three keystrokes; each resets the debounce timer.
    await user.type(searchInput, "ali");

    // Advance past the 300ms debounce window.
    vi.advanceTimersByTime(300);

    // Restore real timers before waiting for DOM updates.
    vi.useRealTimers();

    await waitFor(() => {
      expect(screen.getByText("Alpha Deck")).toBeInTheDocument();
    });

    // Exactly one search query — not one per keystroke.
    expect(searchQueryCalls).toBe(1);
  });

  // T3: Infinite-scroll pagination — first page hasNextPage:true; firing the
  // IntersectionObserver triggers a fetchMore with after:<endCursor>; appended rows render.
  test("IntersectionObserver triggers fetchMore and appends the next page", async () => {
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeMaster(i + 1));
    const secondBatch = Array.from({ length: 20 }, (_, i) => makeMaster(i + 21));
    const endCursor = `m-${firstBatch.length}`;

    let nextPageCalls = 0;
    const nextPageResult = vi.fn(() => {
      nextPageCalls += 1;
      return { data: { adminMasters: makeConnection(secondBatch, false) } };
    });

    const mocks = [
      {
        request: {
          query: AdminMastersDocument,
          variables: { ...BASE_VARS, search: null },
        },
        result: { data: { adminMasters: makeConnection(firstBatch, true) } },
      },
      {
        request: {
          query: AdminMastersDocument,
          variables: { ...BASE_VARS, after: endCursor, search: null },
        },
        delay: 50,
        result: nextPageResult,
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminMastersDocument,
      variables: { ...BASE_VARS, search: null },
      data: { adminMasters: makeConnection(firstBatch, true) },
    });

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminMastersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("Deck m-1")).toBeInTheDocument();

    // Fire the observer twice synchronously — the in-flight ref guard collapses
    // the pair into a single fetchMore.
    fireIntersect();
    fireIntersect();

    await waitFor(() => {
      expect(screen.getByText("Deck m-21")).toBeInTheDocument();
    });

    // The first batch is still present (appended, not replaced).
    expect(screen.getByText("Deck m-1")).toBeInTheDocument();
    expect(nextPageCalls).toBe(1);
  });

  // T4: fetchMore error + retry — the first fetchMore errors, the error banner
  // appears, and clicking Retry re-issues the request and recovers.
  test("fetchMore error shows the banner and Retry re-issues the request", async () => {
    const user = userEvent.setup({ delay: null });
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeMaster(i + 1));
    const secondBatch = Array.from({ length: 20 }, (_, i) => makeMaster(i + 21));
    const endCursor = `m-${firstBatch.length}`;

    let fetchMoreCallCount = 0;

    const failResult = vi.fn(() => {
      fetchMoreCallCount += 1;
      return {
        errors: [{ message: "Could not load more masters. Please try again." }],
      };
    });

    const successResult = vi.fn(() => {
      fetchMoreCallCount += 1;
      return { data: { adminMasters: makeConnection(secondBatch, false) } };
    });

    const fetchMoreVars = { ...BASE_VARS, after: endCursor, search: null };

    const mocks = [
      {
        request: {
          query: AdminMastersDocument,
          variables: { ...BASE_VARS, search: null },
        },
        result: { data: { adminMasters: makeConnection(firstBatch, true) } },
      },
      // First fetchMore fails.
      {
        request: { query: AdminMastersDocument, variables: fetchMoreVars },
        result: failResult,
      },
      // After Retry, the next fetchMore succeeds.
      {
        request: { query: AdminMastersDocument, variables: fetchMoreVars },
        result: successResult,
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminMastersDocument,
      variables: { ...BASE_VARS, search: null },
      data: { adminMasters: makeConnection(firstBatch, true) },
    });

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminMastersClient />
      </MockedProvider>,
    );

    expect(await screen.findByText("Deck m-1")).toBeInTheDocument();

    // Fire the observer — the first fetchMore fails and the banner appears.
    fireIntersect();

    const banner = await screen.findByTestId("admin-masters-fetch-more-error");
    expect(banner).toBeInTheDocument();
    expect(fetchMoreCallCount).toBe(1);

    // While the error banner is present, firing the observer again must NOT
    // issue a fetchMore (the IO loop is halted).
    fireIntersect();
    fireIntersect();
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(fetchMoreCallCount).toBe(1);

    // Click Retry — the banner clears and the second (success) request fires.
    await user.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("Deck m-21")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("admin-masters-fetch-more-error")).not.toBeInTheDocument();
    expect(fetchMoreCallCount).toBe(2);
  });

  // T5: Empty state — an empty connection renders the empty-state copy and hides the list.
  test("renders empty-state copy when the connection has no edges", async () => {
    const connection = makeConnection([], false);

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: AdminMastersDocument,
      variables: { ...BASE_VARS, search: null },
      data: { adminMasters: connection },
    });

    const mocks = [
      {
        request: {
          query: AdminMastersDocument,
          variables: { ...BASE_VARS, search: null },
        },
        result: { data: { adminMasters: connection } },
      },
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <AdminMastersClient />
      </MockedProvider>,
    );

    const empty = await screen.findByTestId("admin-masters-empty");
    expect(empty).toHaveTextContent("No masters found.");
    expect(screen.queryByTestId("admin-masters-list")).not.toBeInTheDocument();
  });
});
