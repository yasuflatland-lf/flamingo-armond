// @vitest-environment jsdom
import { gql, InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// CardsClient reads usePathname for its pending-delete flush effect. Stub it
// here because this test file only exercises bulk delete, not navigation.
vi.mock("next/navigation", () => ({
  usePathname: () => "/cardgroups/cg-bulk-1/edit",
}));
// next/link → plain anchor in jsdom (the empty-state CTA renders a Link).
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

import { CardsClient } from "@/app/cardgroups/[id]/cards/cards-client";
import { CardsByCardgroupConnectionDocument, DeleteCardsDocument } from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";

const CG_ID = "cg-bulk-1";
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

function makeConnection(cards: Card[], hasNextPage = false) {
  return {
    __typename: "CardConnection" as const,
    edges: cards.map(makeEdge),
    pageInfo: {
      __typename: "PageInfo" as const,
      hasNextPage,
      hasPreviousPage: false,
      startCursor: cards[0]?.id ?? null,
      endCursor: cards[cards.length - 1]?.id ?? null,
    },
    totalCount: cards.length,
  };
}

// Stub IntersectionObserver so pagination hooks don't interfere.
class FakeIntersectionObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }
}

beforeEach(() => {
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

// Helper: render CardsClient with a seeded cache and given MockedProvider mocks.
function renderCardsClient(cards: Card[], mocks: object[]) {
  const connection = makeConnection(cards);
  const cache = new InMemoryCache();
  cache.writeQuery({
    query: CardsByCardgroupConnectionDocument,
    // Variables shape MUST match cardsDefaultVars(CG_ID) — cardgroupId, first,
    // and `search: null`. Omitting `search` silently splits the cache key and
    // the bulk-delete update callback's readQuery returns null.
    variables: { cardgroupId: CG_ID, first: PAGE_SIZE, search: null },
    data: { cardsByCardgroupConnection: connection },
  });

  render(
    <MockedProvider mocks={mocks as never} cache={cache}>
      <UndoDeleteProvider>
        <CardsClient
          cardgroupId={CG_ID}
          cardgroupName="Test Cardgroup"
          initialEdges={connection.edges}
          initialPageInfo={connection.pageInfo}
          initialTotalCount={connection.totalCount}
        />
      </UndoDeleteProvider>
    </MockedProvider>,
  );
}

describe("CardsClient — bulk delete", () => {
  // T1: no selection → bulk action bar is not rendered.
  it("does not show the bulk action bar on initial render", () => {
    const cards = [makeCard(1), makeCard(2), makeCard(3)];
    renderCardsClient(cards, []);

    expect(screen.queryByTestId("cards-bulk-action-bar")).not.toBeInTheDocument();
  });

  // T2: selecting two cards shows the bulk action bar with "2 selected".
  it("shows the bulk action bar with correct count after selecting two cards", async () => {
    const user = userEvent.setup();
    const cards = [makeCard(1), makeCard(2), makeCard(3)];
    renderCardsClient(cards, []);

    const cb1 = screen.getByTestId("card-select-c-1");
    const cb2 = screen.getByTestId("card-select-c-2");

    await user.click(cb1);
    await user.click(cb2);

    const bar = screen.getByTestId("cards-bulk-action-bar");
    expect(bar).toBeInTheDocument();
    expect(bar).toHaveTextContent("2 selected");
  });

  // T3: clicking "Delete selected" opens the confirmation dialog.
  it("opens the confirmation dialog when Delete selected is clicked", async () => {
    const user = userEvent.setup();
    const cards = [makeCard(1), makeCard(2)];
    renderCardsClient(cards, []);

    await user.click(screen.getByTestId("card-select-c-1"));
    await user.click(screen.getByTestId("card-select-c-2"));

    await user.click(screen.getByTestId("cards-bulk-delete-button"));

    // The AlertDialog renders into a portal; role=alertdialog is the accessible role.
    expect(await screen.findByRole("alertdialog")).toBeInTheDocument();
    expect(screen.getByText("Delete 2 cards?")).toBeInTheDocument();
    expect(screen.getByText("This action cannot be undone.")).toBeInTheDocument();
  });

  // T4: confirming the dialog fires the mutation; rows are removed and totalCount decrements.
  it("fires the mutation on confirm; rows disappear and totalCount decrements", async () => {
    const user = userEvent.setup();
    const cards = [makeCard(1), makeCard(2), makeCard(3)];

    let mutationCalled = false;
    const mocks = [
      {
        request: {
          query: DeleteCardsDocument,
          variables: { ids: ["c-1", "c-2"] },
        },
        result: () => {
          mutationCalled = true;
          // Backend deleted 2 rows.
          return { data: { deleteCards: 2 } };
        },
      },
    ];

    renderCardsClient(cards, mocks);

    // Select two cards.
    await user.click(screen.getByTestId("card-select-c-1"));
    await user.click(screen.getByTestId("card-select-c-2"));

    // Open confirmation dialog.
    await user.click(screen.getByTestId("cards-bulk-delete-button"));
    await screen.findByRole("alertdialog");

    // Confirm deletion.
    await user.click(screen.getByTestId("cards-bulk-confirm"));

    await waitFor(() => {
      expect(mutationCalled).toBe(true);
    });

    // Deleted rows are gone from the list.
    await waitFor(() => {
      expect(screen.queryByText("front-1")).not.toBeInTheDocument();
      expect(screen.queryByText("front-2")).not.toBeInTheDocument();
    });

    // Surviving card still present.
    expect(screen.getByText("front-3")).toBeInTheDocument();

    // totalCount decremented from 3 → 1 (backend reported 2 deleted).
    expect(screen.getByText("Cards (1)")).toBeInTheDocument();

    // Bulk action bar is cleared after a successful delete.
    expect(screen.queryByTestId("cards-bulk-action-bar")).not.toBeInTheDocument();
  });

  // T5: clicking Cancel clears the selection without firing the mutation.
  it("clears selection when the Cancel button in the action bar is clicked", async () => {
    const user = userEvent.setup();
    const cards = [makeCard(1), makeCard(2)];

    // Provide no mutation mock; if the mutation fires it will warn.
    renderCardsClient(cards, []);

    await user.click(screen.getByTestId("card-select-c-1"));
    await user.click(screen.getByTestId("card-select-c-2"));
    expect(screen.getByTestId("cards-bulk-action-bar")).toBeInTheDocument();

    // Click the Cancel button in the bulk action bar.
    await user.click(screen.getByRole("button", { name: /^cancel$/i }));

    // Bulk action bar disappears; no mutation was fired.
    expect(screen.queryByTestId("cards-bulk-action-bar")).not.toBeInTheDocument();
  });

  // T7: backend returns deleteCards === 0 → no-op; cards and count are unchanged.
  it("leaves cards and count unchanged when backend returns deleteCards 0", async () => {
    const user = userEvent.setup();
    const cards = [makeCard(1), makeCard(2), makeCard(3)];

    const evictSpy = vi.fn();
    const cache = new InMemoryCache();
    cache.evict = evictSpy;
    const connection = makeConnection(cards);
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
      data: { cardsByCardgroupConnection: connection },
    });

    const mocks = [
      {
        request: {
          query: DeleteCardsDocument,
          variables: { ids: ["c-1", "c-2"] },
        },
        result: { data: { deleteCards: 0 } },
      },
    ];

    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <UndoDeleteProvider>
          <CardsClient
            cardgroupId={CG_ID}
            cardgroupName="Test Cardgroup"
            initialEdges={connection.edges}
            initialPageInfo={connection.pageInfo}
            initialTotalCount={connection.totalCount}
          />
        </UndoDeleteProvider>
      </MockedProvider>,
    );

    // Select two cards and confirm deletion.
    await user.click(screen.getByTestId("card-select-c-1"));
    await user.click(screen.getByTestId("card-select-c-2"));
    await user.click(screen.getByTestId("cards-bulk-delete-button"));
    await screen.findByRole("alertdialog");
    await user.click(screen.getByTestId("cards-bulk-confirm"));

    // Action bar is cleared on success (clearSelection was called after mutation resolves).
    await waitFor(() => {
      expect(screen.queryByTestId("cards-bulk-action-bar")).not.toBeInTheDocument();
    });

    // All three cards are still present and count is unchanged.
    expect(screen.getByText("Cards (3)")).toBeInTheDocument();
    expect(screen.getByText("front-1")).toBeInTheDocument();
    expect(screen.getByText("front-2")).toBeInTheDocument();
    expect(screen.getByText("front-3")).toBeInTheDocument();

    // No error banner.
    expect(screen.queryByTestId("cards-bulk-delete-error")).not.toBeInTheDocument();

    // cache.evict must NOT have been called — the bug fix ensures this.
    expect(evictSpy).not.toHaveBeenCalled();
  });

  // T8: cold-cache (no connection entry) — writeQuery is skipped, eviction still fires.
  it("skips writeQuery on cold cache and evicts the deleted card entry", async () => {
    const user = userEvent.setup();
    const cards = [makeCard(1), makeCard(2), makeCard(3)];

    // Build a cache without seeding the connection query — only the SSR-seeded
    // initialEdges/initialPageInfo props back the UI on first paint.
    const cache = new InMemoryCache();
    const connection = makeConnection(cards);

    const mocks = [
      {
        request: {
          query: DeleteCardsDocument,
          variables: { ids: ["c-1"] },
        },
        result: { data: { deleteCards: 1 } },
      },
    ];

    let caughtError: unknown = null;
    render(
      <MockedProvider mocks={mocks as never} cache={cache}>
        <UndoDeleteProvider>
          <CardsClient
            cardgroupId={CG_ID}
            cardgroupName="Test Cardgroup"
            initialEdges={connection.edges}
            initialPageInfo={connection.pageInfo}
            initialTotalCount={connection.totalCount}
          />
        </UndoDeleteProvider>
      </MockedProvider>,
    );

    // Select one card and confirm.
    await user.click(screen.getByTestId("card-select-c-1"));
    await user.click(screen.getByTestId("cards-bulk-delete-button"));
    await screen.findByRole("alertdialog");

    try {
      await user.click(screen.getByTestId("cards-bulk-confirm"));
    } catch (err) {
      caughtError = err;
    }

    // No exception thrown.
    expect(caughtError).toBeNull();

    // Wait for mutation to complete.
    await waitFor(() => {
      expect(screen.queryByTestId("cards-bulk-action-bar")).not.toBeInTheDocument();
    });

    // The connection was never seeded, so readQuery returns null — cache stays empty.
    const result = cache.readQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
    });
    expect(result).toBeNull();

    // The normalized Card entry for c-1 was evicted (no data in cache for it).
    const cardResult = cache.readFragment({
      id: cache.identify({ __typename: "Card", id: "c-1" }),
      fragment: gql`
        fragment T8Card on Card {
          id
        }
      `,
    });
    expect(cardResult).toBeNull();
  });

  // T6: backend error → banner appears; selection remains intact.
  it("shows error banner on mutation failure and preserves the selection", async () => {
    const user = userEvent.setup();
    const cards = [makeCard(1), makeCard(2)];

    const mocks = [
      {
        request: {
          query: DeleteCardsDocument,
          variables: { ids: ["c-1", "c-2"] },
        },
        result: {
          errors: [new GraphQLError("permission denied", { extensions: { code: "FORBIDDEN" } })],
        },
      },
    ];

    renderCardsClient(cards, mocks);

    await user.click(screen.getByTestId("card-select-c-1"));
    await user.click(screen.getByTestId("card-select-c-2"));

    await user.click(screen.getByTestId("cards-bulk-delete-button"));
    await screen.findByRole("alertdialog");
    await user.click(screen.getByTestId("cards-bulk-confirm"));

    // Error banner appears.
    const banner = await screen.findByTestId("cards-bulk-delete-error");
    expect(banner).toBeInTheDocument();

    // Selection persists (action bar still visible with 2 selected).
    await waitFor(() => {
      expect(screen.getByTestId("cards-bulk-action-bar")).toHaveTextContent("2 selected");
    });

    // Both cards still rendered.
    expect(screen.getByText("front-1")).toBeInTheDocument();
    expect(screen.getByText("front-2")).toBeInTheDocument();
  });
});
