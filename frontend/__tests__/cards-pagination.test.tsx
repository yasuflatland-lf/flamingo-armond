// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CardsClient } from "@/app/cardgroups/[id]/cards/cards-client";
import { CardsByCardgroupConnectionDocument } from "@/generated/graphql";

const CG_ID = "cg-1";
const PAGE_SIZE = 20;

type Card = {
  __typename: "Card";
  id: string;
  front: string;
  back: string;
  due: string;
  state: number;
  cardgroupId: string;
};

function makeCard(i: number): Card {
  return {
    __typename: "Card",
    id: `c-${i}`,
    front: `front-${i}`,
    back: `back-${i}`,
    due: "2024-06-15",
    state: 0,
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
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

function fireIntersect() {
  const cb = ioCallbacks[ioCallbacks.length - 1];
  if (!cb) throw new Error("No IntersectionObserver registered");
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
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
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
          variables: { cardgroupId: CG_ID, first: PAGE_SIZE, after: "c-20" },
        },
        result: nextPageResult,
      },
    ];

    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
      data: { cardsByCardgroupConnection: makeConnection(firstBatch, true) },
    });

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <CardsClient
          cardgroupId={CG_ID}
          initialEdges={initialEdges}
          initialPageInfo={initialPageInfo}
          initialTotalCount={initialTotalCount}
        />
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
});
