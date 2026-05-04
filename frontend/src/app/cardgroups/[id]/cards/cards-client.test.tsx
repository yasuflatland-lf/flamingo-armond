// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  CardsByCardgroupConnectionDocument,
  DeleteCardDocument,
  UpdateCardDocument,
} from "@/generated/graphql";
import { CardsClient } from "./cards-client";

const CG_ID = "cg-1";
const PAGE_SIZE = 20;

const CARD_1 = {
  __typename: "Card" as const,
  id: "c-1",
  front: "Hello",
  back: "Hola",
  due: "2024-06-15",
  state: 0,
  cardgroupId: CG_ID,
};

const CARD_2 = {
  __typename: "Card" as const,
  id: "c-2",
  front: "Bye",
  back: "Adios",
  due: "2024-06-15",
  state: 0,
  cardgroupId: CG_ID,
};

function edge(card: typeof CARD_1) {
  return {
    __typename: "CardEdge" as const,
    cursor: card.id,
    node: card,
  };
}

function connection(
  cards: ReadonlyArray<typeof CARD_1>,
  hasNext = false,
): {
  __typename: "CardConnection";
  edges: ReturnType<typeof edge>[];
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

function defaultPageInfo(edges: ReturnType<typeof edge>[]) {
  return {
    __typename: "PageInfo" as const,
    hasNextPage: false,
    hasPreviousPage: false,
    startCursor: edges[0]?.cursor ?? null,
    endCursor: edges[edges.length - 1]?.cursor ?? null,
  };
}

function makeUpdateMock(id: string, input: { front: string; back: string }, card = CARD_1) {
  return {
    request: { query: UpdateCardDocument, variables: { id, input } },
    result: {
      data: {
        updateCard: { __typename: "UpdateCardPayload" as const, card },
      },
    },
  };
}

function makeDeleteMock(id: string) {
  return {
    request: { query: DeleteCardDocument, variables: { id } },
    result: { data: { deleteCard: true } },
  };
}

function renderClient(
  mocks: unknown[],
  initialCards = [CARD_1, CARD_2],
  options: { errorPolicy?: boolean; cache?: InMemoryCache } = {},
) {
  const defaultOptions = options.errorPolicy
    ? { mutate: { errorPolicy: "all" as const } }
    : undefined;

  const initialEdges = initialCards.map(edge);

  render(
    <MockedProvider mocks={mocks as never} defaultOptions={defaultOptions} cache={options.cache}>
      <CardsClient
        cardgroupId={CG_ID}
        initialEdges={initialEdges}
        initialPageInfo={defaultPageInfo(initialEdges)}
        initialTotalCount={initialEdges.length}
      />
    </MockedProvider>,
  );
}

// Stub IntersectionObserver since cards-client wires one up in a useEffect.
beforeEach(() => {
  vi.stubGlobal(
    "IntersectionObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
      takeRecords() {
        return [];
      }
    },
  );
});

describe("<CardsClient>", () => {
  it("renders existing cards", () => {
    renderClient([]);
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("Bye")).toBeInTheDocument();
  });

  it("create form and Add a card heading are not present", () => {
    renderClient([]);
    expect(screen.queryByText(/add a card/i)).toBeNull();
    expect(screen.queryByRole("button", { name: /^add$/i })).toBeNull();
  });

  it("edit toggle shows edit form and cancel reverts to view", async () => {
    const user = userEvent.setup();
    renderClient([]);

    const firstEditBtn = screen.getAllByRole("button", { name: /edit/i })[0] as HTMLElement;
    await user.click(firstEditBtn);

    const editFrontInput = screen.getByLabelText(/front/i) as HTMLElement;
    expect(editFrontInput).toHaveValue("Hello");

    await user.click(screen.getByRole("button", { name: /cancel/i }));

    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /cancel/i })).not.toBeInTheDocument();
    });
    expect(screen.getByText("Hello")).toBeInTheDocument();
  });

  it("edit save calls updateCard mutation and shows updated values", async () => {
    const user = userEvent.setup();
    const updatedCard = { ...CARD_1, front: "Hello updated", back: "Hola updated" };
    const mock = makeUpdateMock(
      "c-1",
      { front: "Hello updated", back: "Hola updated" },
      updatedCard,
    );

    // Seed cache so cache normalization can update the edge node when mutation completes.
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    renderClient([mock], [CARD_1, CARD_2], { cache });

    const firstEditBtn = screen.getAllByRole("button", { name: /edit/i })[0] as HTMLElement;
    await user.click(firstEditBtn);

    const editFrontInput = screen.getByLabelText(/front/i) as HTMLElement;
    await user.clear(editFrontInput);
    await user.type(editFrontInput, "Hello updated");

    const editBackInput = screen.getByLabelText(/back/i) as HTMLElement;
    await user.clear(editBackInput);
    await user.type(editBackInput, "Hola updated");

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByText("Hello updated")).toBeInTheDocument();
    });
  });

  it("delete dialog confirm removes the row and calls mutation", async () => {
    const user = userEvent.setup();
    const mock = makeDeleteMock("c-1");

    // Seed a real InMemoryCache so the update callback's modify/evict/gc is exercised.
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
      data: { cardsByCardgroupConnection: connection([CARD_1, CARD_2]) },
    });

    renderClient([mock], [CARD_1, CARD_2], { cache });

    expect(screen.getByText("Hello")).toBeInTheDocument();

    const firstDeleteBtn = screen.getAllByRole("button", { name: /delete/i })[0] as HTMLElement;
    await user.click(firstDeleteBtn);

    const dialog = await screen.findByRole("alertdialog");
    const confirmBtn = within(dialog).getByRole("button", { name: /delete/i });
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(screen.queryByText("Hello")).not.toBeInTheDocument();
    });

    const cached = cache.readQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
    });
    const ids = cached?.cardsByCardgroupConnection.edges.map((e) => e.node.id) ?? [];
    expect(ids).not.toContain(CARD_1.id);
  });

  it("UNAUTHENTICATED on delete shows banner", async () => {
    const user = userEvent.setup();

    const mock = {
      request: { query: DeleteCardDocument, variables: { id: "c-1" } },
      result: {
        errors: [new GraphQLError("Unauthenticated", { extensions: { code: "UNAUTHENTICATED" } })],
      },
    };

    renderClient([mock], [CARD_1], { errorPolicy: true });

    const deleteBtn = screen.getByRole("button", { name: /delete/i });
    await user.click(deleteBtn);

    const dialog = await screen.findByRole("alertdialog");
    const confirmBtn = within(dialog).getByRole("button", { name: /delete/i });
    await user.click(confirmBtn);

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
  });

  it("edit save failure keeps the row in edit mode and shows the inline error", async () => {
    const user = userEvent.setup();

    const mock = {
      request: {
        query: UpdateCardDocument,
        variables: { id: "c-1", input: { front: "", back: "Hola" } },
      },
      result: {
        errors: [
          new GraphQLError("front is required", {
            extensions: { code: "BAD_USER_INPUT", field: "front" },
          }),
        ],
      },
    };

    renderClient([mock], [CARD_1], { errorPolicy: true });

    const editBtn = screen.getByRole("button", { name: /edit/i });
    await user.click(editBtn);

    const editFrontInput = screen.getByLabelText(/front/i) as HTMLElement;
    await user.clear(editFrontInput);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(screen.getByText("front is required")).toBeInTheDocument();
    });

    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^save$/i })).toBeInTheDocument();
  });

  // G1: useQuery error surfaces a banner via the cards-query-error testid.
  it("useQuery error renders banner with cards-query-error testid", async () => {
    const queryErrorMock = {
      request: {
        query: CardsByCardgroupConnectionDocument,
        variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
      },
      result: {
        errors: [new GraphQLError("boom", { extensions: { code: "INTERNAL" } })],
      },
    };

    renderClient([queryErrorMock], [CARD_1, CARD_2]);

    const banner = await screen.findByTestId("cards-query-error");
    // INTERNAL errors surface with the original message via getBackendErrorBanner.
    expect(banner).toHaveTextContent("boom");
  });

  // G5: delete decrements totalCount unconditionally — even when the filter is a
  // no-op because the deleted card lives on a page that was never fetched into
  // the cached edges. Strategy: open the delete dialog while the card is still
  // visible, then mutate the cache to drop that card's edge before confirming —
  // the dialog's onClick captured card.id in closure, so the mutation still
  // fires for the original id, and the deleteCard.update callback now sees a
  // cached connection that does NOT contain that id.
  it("delete decrements totalCount even when card is not in cached edges", async () => {
    const user = userEvent.setup();
    const mock = makeDeleteMock("c-1");

    // Seed cache with BOTH edges so CARD_1's Delete button renders.
    const cache = new InMemoryCache();
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
      data: {
        cardsByCardgroupConnection: {
          __typename: "CardConnection" as const,
          edges: [edge(CARD_1), edge(CARD_2)],
          pageInfo: {
            __typename: "PageInfo" as const,
            hasNextPage: true,
            hasPreviousPage: false,
            startCursor: CARD_1.id,
            endCursor: CARD_2.id,
          },
          totalCount: 5,
        },
      },
    });

    renderClient([mock], [CARD_1, CARD_2], { cache });

    // Open the delete dialog for CARD_1 while it's still visible.
    const firstDeleteBtn = screen.getAllByRole("button", { name: /delete/i })[0] as HTMLElement;
    await user.click(firstDeleteBtn);

    const dialog = await screen.findByRole("alertdialog");
    const confirmBtn = within(dialog).getByRole("button", { name: /delete/i });

    // Drop CARD_1 from the cached connection (keep totalCount=5) WITHOUT
    // broadcasting so the component does not re-render and unmount the dialog.
    // The deleteCard.update callback will then read an existing connection whose
    // edges do not include CARD_1, exercising the no-op-filter path.
    cache.writeQuery({
      query: CardsByCardgroupConnectionDocument,
      variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
      data: {
        cardsByCardgroupConnection: {
          __typename: "CardConnection" as const,
          edges: [edge(CARD_2)],
          pageInfo: {
            __typename: "PageInfo" as const,
            hasNextPage: true,
            hasPreviousPage: false,
            startCursor: CARD_2.id,
            endCursor: CARD_2.id,
          },
          totalCount: 5,
        },
      },
      broadcast: false,
    });

    // Confirm the delete — the dialog's onClick closure still uses card.id="c-1".
    await user.click(confirmBtn);

    await waitFor(() => {
      const cached = cache.readQuery({
        query: CardsByCardgroupConnectionDocument,
        variables: { cardgroupId: CG_ID, first: PAGE_SIZE },
      });
      // totalCount must drop from 5 to 4 even though the filter removed nothing.
      // A gated-decrement regression (only decrement when filter actually
      // removes an edge) would leave totalCount at 5 here.
      expect(cached?.cardsByCardgroupConnection.totalCount).toBe(4);
    });
  });
});
