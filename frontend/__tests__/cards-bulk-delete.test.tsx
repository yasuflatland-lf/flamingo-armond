// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CardsClient } from "@/app/cardgroups/[id]/cards/cards-client";
import { CardsByCardgroupConnectionDocument, DeleteCardsDocument } from "@/generated/graphql";

const CG_ID = "cg-bulk-1";
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
    variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
    data: { cardsByCardgroupConnection: connection },
  });

  render(
    <MockedProvider mocks={mocks as never} cache={cache}>
      <CardsClient
        cardgroupId={CG_ID}
        initialEdges={connection.edges}
        initialPageInfo={connection.pageInfo}
        initialTotalCount={connection.totalCount}
      />
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
