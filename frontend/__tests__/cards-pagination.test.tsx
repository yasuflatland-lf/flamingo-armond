// @vitest-environment happy-dom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// CardsClient reads usePathname for its pending-delete flush effect. Stub it
// here because this test file only exercises pagination, not navigation.
vi.mock("next/navigation", () => ({
  usePathname: () => "/cardgroups/cg-1/edit",
}));
// next/link → plain anchor in jsdom (empty-state CTA renders a Link).
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

import { NextIntlClientProvider } from "next-intl";
import { CardsClient } from "@/app/cardgroups/[id]/cards/cards-client";
import { CardsByCardgroupConnectionDocument } from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import enMessages from "../messages/en.json";

const CG_ID = "cg-1";
const PAGE_SIZE = 20;

type Card = {
  __typename: "Card";
  id: string;
  front: string;
  back: string;
  userCardState: {
    __typename: "UserCardState";
    due: string;
    state: number;
  };
  cardgroupId: string;
};

function makeCard(i: number): Card {
  return {
    __typename: "Card",
    id: `c-${i}`,
    front: `front-${i}`,
    back: `back-${i}`,
    userCardState: {
      __typename: "UserCardState",
      due: "2024-06-15",
      state: 0,
    },
    cardgroupId: CG_ID,
  };
}

function makeEdge(card: Card) {
  return {
    __typename: "CardEdge" as const,
    cursor: card.id,
    node: card,
  };
}

function makeConnection(
  cards: Card[],
  hasNextPage: boolean,
): {
  __typename: "CardConnection";
  edges: ReturnType<typeof makeEdge>[];
  pageInfo: {
    __typename: "PageInfo";
    hasNextPage: boolean;
    hasPreviousPage: boolean;
    startCursor: string | null;
    endCursor: string | null;
  };
  totalCount: number;
} {
  return {
    __typename: "CardConnection",
    edges: cards.map(makeEdge),
    pageInfo: {
      __typename: "PageInfo",
      hasNextPage,
      hasPreviousPage: false,
      startCursor: cards[0]?.id ?? null,
      endCursor: cards[cards.length - 1]?.id ?? null,
    },
    totalCount: cards.length,
  };
}

// Capture the most recent IntersectionObserver callback so the test can fire it manually.
let ioCallbacks: IntersectionObserverCallback[] = [];

class FakeIntersectionObserver {
  callback: IntersectionObserverCallback;
  constructor(cb: IntersectionObserverCallback) {
    this.callback = cb;
    ioCallbacks.push(cb);
  }
  observe() {}
  unobserve() {}
  // Real browsers stop firing the callback after disconnect; mirror that here so
  // stale-closure observers don't keep firing fetchMore after the production
  // useEffect cleanup runs.
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
  // The component's callback only inspects entries[0].isIntersecting.
  cb([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver);
}

beforeEach(() => {
  ioCallbacks = [];
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("CardsClient pagination via IntersectionObserver", () => {
  it("fetches the next page when the sentinel intersects, then stops once hasNextPage is false", async () => {
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeCard(i + 1));
    const secondBatch = Array.from({ length: 20 }, (_, i) => makeCard(i + 21));

    const initialEdges = firstBatch.map(makeEdge);
    const initialPageInfo = {
      __typename: "PageInfo" as const,
      hasNextPage: true,
      hasPreviousPage: false,
      startCursor: "c-1",
      endCursor: "c-20",
    };
    const initialTotalCount = 40;

    // Track how many times the next-page mock is invoked. MockedProvider results
    // are consumed once per matching request, so a second call without another
    // mock would warn — that itself is the assertion-of-no-second-call.
    let nextPageCalls = 0;
    const nextPageResult = vi.fn(() => {
      nextPageCalls += 1;
      return {
        data: {
          cardsByCardgroupConnection: makeConnection(secondBatch, false),
        },
      };
    });

    const mocks = [
      // First useQuery call resolves from cache (we seed the cache below) so no
      // network mock for the initial { first: 20 } request is strictly needed,
      // but provide one defensively in case Apollo refetches on mount.
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
        },
        result: {
          data: {
            cardsByCardgroupConnection: makeConnection(firstBatch, true),
          },
        },
      },
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, after: "c-20", search: null },
        },
        result: nextPageResult,
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
      data: { cardsByCardgroupConnection: makeConnection(firstBatch, true) },
    });

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
          <UndoDeleteProvider>
            <CardsClient
              cardgroupId={CG_ID}
              cardgroupName="Test Cardgroup"
              initialEdges={initialEdges}
              initialPageInfo={initialPageInfo}
              initialTotalCount={initialTotalCount}
            />
          </UndoDeleteProvider>
        </NextIntlClientProvider>
      </MockedProvider>,
    );

    // Initial render shows the first batch.
    expect(screen.getByText("front-1")).toBeInTheDocument();
    expect(screen.getByText("front-20")).toBeInTheDocument();
    expect(screen.queryByText("front-21")).not.toBeInTheDocument();

    // Fire the observer once — fetchMore should run and append the next batch.
    fireIntersect();

    await waitFor(() => {
      expect(screen.getByText("front-21")).toBeInTheDocument();
      expect(screen.getByText("front-40")).toBeInTheDocument();
    });
    expect(nextPageCalls).toBe(1);

    // Fire again — hasNextPage is now false, so no further fetchMore should run.
    fireIntersect();

    // Give any pending microtasks a chance to flush.
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(nextPageCalls).toBe(1);
  });

  it("does not double-fire fetchMore while a previous request is in flight", async () => {
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeCard(i + 1));
    const secondBatch = Array.from({ length: 20 }, (_, i) => makeCard(i + 21));

    const initialEdges = firstBatch.map(makeEdge);
    const initialPageInfo = {
      __typename: "PageInfo" as const,
      hasNextPage: true,
      hasPreviousPage: false,
      startCursor: "c-1",
      endCursor: "c-20",
    };
    const initialTotalCount = 40;

    // Use a finite delay so the request is briefly in-flight; the result function
    // is called once-per-request, so we can assert it ran exactly once.
    let nextPageCalls = 0;
    const nextPageResult = vi.fn(() => {
      nextPageCalls += 1;
      return { data: { cardsByCardgroupConnection: makeConnection(secondBatch, false) } };
    });

    const mocks = [
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
        },
        result: {
          data: { cardsByCardgroupConnection: makeConnection(firstBatch, true) },
        },
      },
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, after: "c-20", search: null },
        },
        delay: 50,
        result: nextPageResult,
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
      data: { cardsByCardgroupConnection: makeConnection(firstBatch, true) },
    });

    // Spy on console.warn to detect "no more mocked responses" warnings emitted
    // by MockedProvider when an unmatched query escapes the in-flight guard.
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
          <UndoDeleteProvider>
            <CardsClient
              cardgroupId={CG_ID}
              cardgroupName="Test Cardgroup"
              initialEdges={initialEdges}
              initialPageInfo={initialPageInfo}
              initialTotalCount={initialTotalCount}
            />
          </UndoDeleteProvider>
        </NextIntlClientProvider>
      </MockedProvider>,
    );

    expect(screen.getByText("front-20")).toBeInTheDocument();

    // Synchronously fire the observer twice — the in-flight guard must
    // suppress the second call so only one fetchMore actually goes out.
    fireIntersect();
    fireIntersect();

    // Wait for the delayed mock to resolve and append the second batch.
    await waitFor(() => {
      expect(screen.getByText("front-40")).toBeInTheDocument();
    });
    expect(nextPageCalls).toBe(1);

    // hasNextPage flipped to false — further intersects must not trigger a new request.
    fireIntersect();
    await new Promise((resolve) => setTimeout(resolve, 60));
    expect(nextPageCalls).toBe(1);

    // No "no more mocked responses for the query: CardsByCardgroupConnection"
    // warning should ever fire — that warning indicates a duplicate request
    // leaked past the in-flight guard. The single mock above is consumed once,
    // so any second call would warn.
    const cardsConnectionWarnings = warnSpy.mock.calls.filter((args) =>
      args.some((arg) => typeof arg === "string" && arg.includes("CardsByCardgroupConnection")),
    );
    expect(cardsConnectionWarnings).toEqual([]);

    warnSpy.mockRestore();
  });

  // G2: a fetchMore rejection must surface a banner and halt the observer loop.
  it("fetchMore error stops the observer and shows the retry banner", async () => {
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeCard(i + 1));

    const initialEdges = firstBatch.map(makeEdge);
    const initialPageInfo = {
      __typename: "PageInfo" as const,
      hasNextPage: true,
      hasPreviousPage: false,
      startCursor: "c-1",
      endCursor: "c-20",
    };
    const initialTotalCount = 40;

    // Counter: assert the next-page request is consumed at most once even if the
    // observer fires multiple times after the failure.
    let nextPageCalls = 0;
    const nextPageResult = vi.fn(() => {
      nextPageCalls += 1;
      return {
        errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })],
      };
    });

    const mocks = [
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
        },
        result: {
          data: { cardsByCardgroupConnection: makeConnection(firstBatch, true) },
        },
      },
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, after: "c-20", search: null },
        },
        result: nextPageResult,
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
      data: { cardsByCardgroupConnection: makeConnection(firstBatch, true) },
    });

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
          <UndoDeleteProvider>
            <CardsClient
              cardgroupId={CG_ID}
              cardgroupName="Test Cardgroup"
              initialEdges={initialEdges}
              initialPageInfo={initialPageInfo}
              initialTotalCount={initialTotalCount}
            />
          </UndoDeleteProvider>
        </NextIntlClientProvider>
      </MockedProvider>,
    );

    fireIntersect();

    const banner = await screen.findByTestId("cards-fetch-more-error");
    expect(banner).toBeInTheDocument();
    expect(nextPageCalls).toBe(1);

    // Observer loop must be halted: more intersects after the error should not
    // re-issue the request.
    fireIntersect();
    fireIntersect();
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(nextPageCalls).toBe(1);
  });

  // G3: clicking Retry clears the banner and re-issues the request; on success
  // the new edges render. A second failure shows a fresh banner.
  it("retry button clears the error and re-issues the request; a second failure shows a fresh banner", async () => {
    const user = userEvent.setup();
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeCard(i + 1));
    const secondBatch = Array.from({ length: 20 }, (_, i) => makeCard(i + 21));

    const initialEdges = firstBatch.map(makeEdge);
    const initialPageInfo = {
      __typename: "PageInfo" as const,
      hasNextPage: true,
      hasPreviousPage: false,
      startCursor: "c-1",
      endCursor: "c-20",
    };
    const initialTotalCount = 40;

    const mocks = [
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
        },
        result: {
          data: { cardsByCardgroupConnection: makeConnection(firstBatch, true) },
        },
      },
      // First fetchMore fails.
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, after: "c-20", search: null },
        },
        result: {
          errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })],
        },
      },
      // Second fetchMore (after retry) succeeds and appends the second batch.
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, after: "c-20", search: null },
        },
        result: {
          data: { cardsByCardgroupConnection: makeConnection(secondBatch, false) },
        },
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
      data: { cardsByCardgroupConnection: makeConnection(firstBatch, true) },
    });

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
          <UndoDeleteProvider>
            <CardsClient
              cardgroupId={CG_ID}
              cardgroupName="Test Cardgroup"
              initialEdges={initialEdges}
              initialPageInfo={initialPageInfo}
              initialTotalCount={initialTotalCount}
            />
          </UndoDeleteProvider>
        </NextIntlClientProvider>
      </MockedProvider>,
    );

    fireIntersect();

    // Banner appears after the first failure.
    await screen.findByTestId("cards-fetch-more-error");

    // Click Retry — should clear the banner and re-issue the request, yielding
    // the second batch.
    await user.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("front-40")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("cards-fetch-more-error")).not.toBeInTheDocument();
  });

  // G3 (failure-then-failure cycle): a fresh banner shows after the second failure.
  it("retry on a second failure shows a fresh banner", async () => {
    const user = userEvent.setup();
    const firstBatch = Array.from({ length: 20 }, (_, i) => makeCard(i + 1));

    const initialEdges = firstBatch.map(makeEdge);
    const initialPageInfo = {
      __typename: "PageInfo" as const,
      hasNextPage: true,
      hasPreviousPage: false,
      startCursor: "c-1",
      endCursor: "c-20",
    };
    const initialTotalCount = 40;

    const mocks = [
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
        },
        result: {
          data: { cardsByCardgroupConnection: makeConnection(firstBatch, true) },
        },
      },
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, after: "c-20", search: null },
        },
        result: {
          errors: [new GraphQLError("boom1", { extensions: { code: "INTERNAL" } })],
        },
      },
      {
        request: {
          query: CardsByCardgroupConnectionDocument,
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, after: "c-20", search: null },
        },
        result: {
          errors: [new GraphQLError("boom2", { extensions: { code: "INTERNAL" } })],
        },
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
      data: { cardsByCardgroupConnection: makeConnection(firstBatch, true) },
    });

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
          <UndoDeleteProvider>
            <CardsClient
              cardgroupId={CG_ID}
              cardgroupName="Test Cardgroup"
              initialEdges={initialEdges}
              initialPageInfo={initialPageInfo}
              initialTotalCount={initialTotalCount}
            />
          </UndoDeleteProvider>
        </NextIntlClientProvider>
      </MockedProvider>,
    );

    fireIntersect();
    const banner1 = await screen.findByTestId("cards-fetch-more-error");
    expect(banner1).toHaveTextContent("boom1");

    await user.click(screen.getByRole("button", { name: /retry/i }));

    // Banner reappears with the second error message.
    await waitFor(() => {
      expect(screen.getByTestId("cards-fetch-more-error")).toHaveTextContent("boom2");
    });
  });
});
